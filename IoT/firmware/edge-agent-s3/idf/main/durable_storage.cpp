#include "authority_limits.h"
#include "durable_storage.h"
#include "authority_unp.h"
#include <limits>
#include "driver/sdmmc_host.h"
#include "esp_vfs_fat.h"
#include "sdmmc_cmd.h"
#include "sqlite3.h"
#include "cJSON.h"
#include "mbedtls/sha256.h"
#include <cstring>
#include <mutex>
#include <set>
#include <sys/stat.h>

namespace beefiscal::idf {namespace{
constexpr const char*kSchema=R"sql(
CREATE TABLE IF NOT EXISTS schema_version(version INTEGER PRIMARY KEY,applied_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS operations(
 operation_id TEXT PRIMARY KEY,payload_digest TEXT NOT NULL,first_transport TEXT NOT NULL,
 route_snapshot TEXT NOT NULL,state TEXT NOT NULL DEFAULT 'RESERVED',result_payload TEXT,
 created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL,
 CHECK(length(operation_id) BETWEEN 1 AND 128),CHECK(length(payload_digest)=64));
CREATE TABLE IF NOT EXISTS receipts(
 receipt_id TEXT PRIMARY KEY,operation_id TEXT NOT NULL REFERENCES operations(operation_id),
 state TEXT NOT NULL,canonical_payload TEXT NOT NULL,created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS payments(
 payment_id TEXT PRIMARY KEY,operation_id TEXT NOT NULL REFERENCES operations(operation_id),
 receipt_id TEXT REFERENCES receipts(receipt_id),state TEXT NOT NULL,amount_minor INTEGER NOT NULL,
 terminal_reference TEXT,rrn TEXT,auth_code TEXT,created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS payment_saga(
 operation_id TEXT PRIMARY KEY REFERENCES operations(operation_id),phase TEXT NOT NULL,updated_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS payment_saga_members(
 operation_id TEXT NOT NULL REFERENCES payment_saga(operation_id),payment_id TEXT PRIMARY KEY REFERENCES payments(payment_id),reversal_reference TEXT,reversal_rrn TEXT,reversal_auth_code TEXT);
CREATE TABLE IF NOT EXISTS terminal_payment_guard(terminal_id TEXT PRIMARY KEY,operation_id TEXT NOT NULL UNIQUE REFERENCES operations(operation_id),created_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS payment_reconciliations(reconciliation_id TEXT PRIMARY KEY,terminal_id TEXT NOT NULL,payment_id TEXT NOT NULL REFERENCES payments(payment_id),operation_id TEXT NOT NULL REFERENCES operations(operation_id),actor_id TEXT NOT NULL,evidence_reference TEXT NOT NULL,evidence_sha256 TEXT NOT NULL,authorization_sha256 TEXT NOT NULL,old_state TEXT NOT NULL,new_state TEXT NOT NULL,created_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS event_outbox(
 sequence INTEGER PRIMARY KEY AUTOINCREMENT,event_id TEXT NOT NULL UNIQUE,
 operation_id TEXT NOT NULL REFERENCES operations(operation_id),kind TEXT NOT NULL,
 payload TEXT NOT NULL,payload_digest TEXT NOT NULL,created_at INTEGER NOT NULL,
 attempt_count INTEGER NOT NULL DEFAULT 0,last_attempt_at INTEGER,last_error TEXT,
 acknowledged_at INTEGER,ack_id TEXT,CHECK(length(payload_digest)=64));
CREATE INDEX IF NOT EXISTS idx_event_pending ON event_outbox(acknowledged_at,sequence);
CREATE INDEX IF NOT EXISTS idx_event_retention ON event_outbox(acknowledged_at,created_at);
CREATE TABLE IF NOT EXISTS sync_cursor(singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 acknowledged_sequence INTEGER NOT NULL DEFAULT 0,ack_id TEXT,committed_event_hash TEXT,
 updated_at INTEGER NOT NULL DEFAULT 0);
INSERT OR IGNORE INTO sync_cursor(singleton)VALUES(1);
CREATE TABLE IF NOT EXISTS frozen_sync_batch(singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 batch_id TEXT NOT NULL,body TEXT NOT NULL,first_sequence INTEGER NOT NULL,last_sequence INTEGER NOT NULL,
 last_event_hash TEXT NOT NULL,previous_hash TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS frozen_sync_members(sequence INTEGER PRIMARY KEY,event_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,kind TEXT NOT NULL,payload TEXT NOT NULL,payload_digest TEXT NOT NULL,created_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS local_sales(
 surrogate_id TEXT PRIMARY KEY,version INTEGER NOT NULL,state TEXT NOT NULL,
 payload TEXT NOT NULL,created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS authority_state(singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 prefix TEXT NOT NULL,range_start INTEGER NOT NULL,range_end INTEGER NOT NULL,
 next_unp INTEGER NOT NULL,next_operation INTEGER NOT NULL,generation INTEGER NOT NULL,
 CHECK(range_start>0),CHECK(range_end>=range_start),CHECK(next_unp>=range_start));
CREATE TABLE IF NOT EXISTS authority_ranges(generation INTEGER PRIMARY KEY,prefix TEXT NOT NULL,range_start INTEGER NOT NULL,range_end INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS receipt_authority(receipt_id TEXT PRIMARY KEY REFERENCES receipts(receipt_id),allocated_unp TEXT NOT NULL,execution_unp TEXT NOT NULL,operation_sequence INTEGER NOT NULL,unp_sequence INTEGER NOT NULL,prefix TEXT NOT NULL,range_start INTEGER NOT NULL,range_end INTEGER NOT NULL,generation INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS sale_unp_claims(unp TEXT PRIMARY KEY,operation_id TEXT NOT NULL UNIQUE REFERENCES operations(operation_id),serial TEXT NOT NULL CHECK(length(serial)=8),sequence INTEGER NOT NULL CHECK(sequence BETWEEN 1 AND 9999999),UNIQUE(serial,sequence),CHECK(length(unp)=21));
INSERT INTO sale_unp_claims(unp,operation_id,serial,sequence) SELECT a.allocated_unp,r.operation_id,substr(a.allocated_unp,1,8),CAST(substr(a.allocated_unp,15,7) AS INTEGER) FROM receipt_authority a JOIN receipts r ON r.receipt_id=a.receipt_id WHERE a.allocated_unp<>'' AND NOT EXISTS(SELECT 1 FROM sale_unp_claims c WHERE c.operation_id=r.operation_id);
INSERT OR IGNORE INTO schema_version(version,applied_at)VALUES(3,CAST(strftime('%s','now')AS INTEGER));
)sql";
struct Statement{sqlite3_stmt*v{};~Statement(){if(v)sqlite3_finalize(v);}};
esp_err_t exec(sqlite3*db,const char*sql){char*message=nullptr;int rc=sqlite3_exec(db,sql,nullptr,nullptr,&message);if(message)sqlite3_free(message);return rc==SQLITE_OK?ESP_OK:ESP_FAIL;}

// Only a completed business outcome may release an uncertain terminal claim.
// A bank approval alone is insufficient when fiscalization/compensation is open.
bool terminal_owner_complete(sqlite3*db,const char*operation){
 Statement q;const char*sql="SELECT o.state,p.state FROM operations o JOIN payment_saga_members m ON m.operation_id=o.operation_id JOIN payments p ON p.payment_id=m.payment_id WHERE o.operation_id=?";
 if(sqlite3_prepare_v2(db,sql,-1,&q.v,nullptr)!=SQLITE_OK)return false;
 sqlite3_bind_text(q.v,1,operation,-1,SQLITE_TRANSIENT);int rc;bool any=false;
 while((rc=sqlite3_step(q.v))==SQLITE_ROW){any=true;const std::string op=(const char*)sqlite3_column_text(q.v,0),payment=(const char*)sqlite3_column_text(q.v,1);
  const bool negative=payment=="CARD_DECLINED"||payment=="CARD_NOT_SENT"||payment=="RESERVED";
  if(!((op=="COMMITTED"&&payment=="CARD_APPROVED")||(op=="COMPENSATED"&&(payment=="REVERSED"||negative))||(op=="REJECTED"&&negative)))return false;
 }return any&&rc==SQLITE_DONE;
}
esp_err_t claim_terminal(sqlite3*db,const char*terminal,const char*operation,int64_t now){
 if(!terminal||!*terminal||strlen(terminal)>128)return ESP_ERR_INVALID_ARG;
 Statement existing;if(sqlite3_prepare_v2(db,"SELECT operation_id FROM terminal_payment_guard WHERE terminal_id=?",-1,&existing.v,nullptr)!=SQLITE_OK)return ESP_FAIL;
 sqlite3_bind_text(existing.v,1,terminal,-1,SQLITE_TRANSIENT);int rc=sqlite3_step(existing.v);
 if(rc==SQLITE_ROW){const std::string owner=(const char*)sqlite3_column_text(existing.v,0);if(!terminal_owner_complete(db,owner.c_str()))return ESP_ERR_INVALID_STATE;
  Statement remove;if(sqlite3_prepare_v2(db,"DELETE FROM terminal_payment_guard WHERE terminal_id=? AND operation_id=?",-1,&remove.v,nullptr)!=SQLITE_OK)return ESP_FAIL;sqlite3_bind_text(remove.v,1,terminal,-1,SQLITE_TRANSIENT);sqlite3_bind_text(remove.v,2,owner.c_str(),-1,SQLITE_TRANSIENT);if(sqlite3_step(remove.v)!=SQLITE_DONE||sqlite3_changes(db)!=1)return ESP_FAIL;
 }else if(rc!=SQLITE_DONE)return ESP_FAIL;
 // Pre-upgrade journals have no guard rows. Never infer their terminal from a
 // mutable current binding: use their immutable operation route snapshot.
 Statement legacy;const char*sql="SELECT DISTINCT o.operation_id,o.route_snapshot FROM payments p JOIN operations o ON o.operation_id=p.operation_id WHERE o.operation_id<>? AND (p.state IN('CARD_SENDING','CARD_UNKNOWN','REVERSAL_SENDING','REVERSAL_UNKNOWN','REVERSAL_FAILED','EXECUTING') OR (p.state='CARD_APPROVED' AND o.state<>'COMMITTED'))";
 if(sqlite3_prepare_v2(db,sql,-1,&legacy.v,nullptr)!=SQLITE_OK)return ESP_FAIL;
 sqlite3_bind_text(legacy.v,1,operation,-1,SQLITE_TRANSIENT);
 while((rc=sqlite3_step(legacy.v))==SQLITE_ROW){const char*owner=(const char*)sqlite3_column_text(legacy.v,0);if(terminal_owner_complete(db,owner))continue;
  auto*route=cJSON_Parse((const char*)sqlite3_column_text(legacy.v,1));auto*id=cJSON_GetObjectItemCaseSensitive(route,"payment_device_id");const bool unknown=!cJSON_IsString(id)||!id->valuestring||!*id->valuestring;const bool same=unknown||std::string(id->valuestring)==terminal;cJSON_Delete(route);if(unknown)return ESP_ERR_NOT_SUPPORTED;if(same)return ESP_ERR_INVALID_STATE;
 }if(rc!=SQLITE_DONE)return ESP_FAIL;
 Statement add;if(sqlite3_prepare_v2(db,"INSERT INTO terminal_payment_guard(terminal_id,operation_id,created_at)VALUES(?,?,?)",-1,&add.v,nullptr)!=SQLITE_OK)return ESP_FAIL;sqlite3_bind_text(add.v,1,terminal,-1,SQLITE_TRANSIENT);sqlite3_bind_text(add.v,2,operation,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(add.v,3,now);return sqlite3_step(add.v)==SQLITE_DONE?ESP_OK:ESP_FAIL;
}
bool hex_digest(const std::string&s){if(s.size()!=64)return false;for(auto c:s)if(!((c>='0'&&c<='9')||(c>='a'&&c<='f')))return false;return true;}
}
struct DurableStorage::Impl{std::recursive_mutex mutex;sqlite3*db{};bool mounted{};std::string mount;sdmmc_card_t*card{};};
DurableStorage::DurableStorage():impl_(new Impl){}DurableStorage::~DurableStorage(){close();delete impl_;}
esp_err_t DurableStorage::open_database_for_test(const char*path){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!path||!*path||impl_->db)return ESP_ERR_INVALID_ARG;if(sqlite3_initialize()!=SQLITE_OK)return ESP_FAIL;if(sqlite3_open_v2(path,&impl_->db,SQLITE_OPEN_READWRITE|SQLITE_OPEN_CREATE|SQLITE_OPEN_FULLMUTEX,nullptr)!=SQLITE_OK){close();return ESP_FAIL;}sqlite3_busy_timeout(impl_->db,5000);if(exec(impl_->db,"PRAGMA journal_mode=DELETE;PRAGMA synchronous=FULL;PRAGMA foreign_keys=ON;")!=ESP_OK||exec(impl_->db,"BEGIN IMMEDIATE;")!=ESP_OK||exec(impl_->db,kSchema)!=ESP_OK||exec(impl_->db,"COMMIT;")!=ESP_OK){exec(impl_->db,"ROLLBACK;");close();return ESP_FAIL;}/* Idempotent forward migration from schema v3 deployed before hash-bound ACKs. */char*message=nullptr;const int migration=sqlite3_exec(impl_->db,"ALTER TABLE sync_cursor ADD COLUMN committed_event_hash TEXT",nullptr,nullptr,&message);if(message)sqlite3_free(message);if(migration!=SQLITE_OK){/* duplicate column is the expected steady state */}return ESP_OK;}
esp_err_t DurableStorage::open(const StorageConfig&c){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!c.mount_point||!c.database_relative_path||c.database_relative_path[0]!='/'||impl_->db)return ESP_ERR_INVALID_ARG;sdmmc_host_t host=SDMMC_HOST_DEFAULT();sdmmc_slot_config_t slot=SDMMC_SLOT_CONFIG_DEFAULT();slot.width=c.one_bit?1:4;if(c.clk_pin>=0&&c.cmd_pin>=0&&c.d0_pin>=0){slot.clk=(gpio_num_t)c.clk_pin;slot.cmd=(gpio_num_t)c.cmd_pin;slot.d0=(gpio_num_t)c.d0_pin;}esp_vfs_fat_sdmmc_mount_config_t mount{.format_if_mount_failed=false,.max_files=8,.allocation_unit_size=16*1024,.disk_status_check_enable=true,.use_one_fat=false};esp_err_t e=esp_vfs_fat_sdmmc_mount(c.mount_point,&host,&slot,&mount,&impl_->card);if(e!=ESP_OK)return e;impl_->mounted=true;impl_->mount=c.mount_point;std::string path=impl_->mount+c.database_relative_path;auto slash=path.rfind('/');if(slash!=std::string::npos&&mkdir(path.substr(0,slash).c_str(),0755)!=0){struct stat st{};if(stat(path.substr(0,slash).c_str(),&st)!=0){close();return ESP_FAIL;}}return open_database_for_test(path.c_str());}
void DurableStorage::close(){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(impl_->db){sqlite3_close_v2(impl_->db);impl_->db=nullptr;}sqlite3_shutdown();if(impl_->mounted){esp_vfs_fat_sdcard_unmount(impl_->mount.c_str(),impl_->card);impl_->mounted=false;impl_->card=nullptr;}impl_->mount.clear();}
bool DurableStorage::ready()const{std::lock_guard<std::recursive_mutex> guard(impl_->mutex);return impl_->db!=nullptr;}
bool DurableStorage::can_accept_operation(int64_t reserve_bytes)const{std::lock_guard<std::recursive_mutex> guard(impl_->mutex);
  if(!ready()||reserve_bytes<65536)return false;
  // ESP-IDF/newlib does not expose statvfs.  The VFS FAT component provides
  // the authoritative free-space query for the mounted SD volume instead.
  // Host-only SQLite tests have no mounted card, so database readiness is the
  // storage admission signal there.
  if(!impl_->mounted)return true;
  uint64_t total_bytes=0,free_bytes=0;
  if(esp_vfs_fat_info(impl_->mount.c_str(),&total_bytes,&free_bytes)!=ESP_OK)return false;
  return free_bytes>=(size_t)reserve_bytes;
}
ReservationResult DurableStorage::reserve(const char*id,const char*digest,const char*transport,const char*route,int64_t now){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);
  if(!ready()||!id||!*id||!digest||strlen(digest)!=64||!transport||!*transport||!route||!*route||now<=0)return{ReservationStatus::Invalid,{}};
  if(exec(impl_->db,"BEGIN IMMEDIATE;")!=ESP_OK)return{ReservationStatus::StorageFailure,{}};
  Statement q;
  if(sqlite3_prepare_v2(impl_->db,"SELECT payload_digest,state,result_payload FROM operations WHERE operation_id=?",-1,&q.v,nullptr)!=SQLITE_OK){exec(impl_->db,"ROLLBACK;");return{ReservationStatus::StorageFailure,{}};}
  sqlite3_bind_text(q.v,1,id,-1,SQLITE_TRANSIENT);
  int rc=sqlite3_step(q.v);
  if(rc==SQLITE_ROW){std::string prior=(const char*)sqlite3_column_text(q.v,0),result;const auto*r=sqlite3_column_text(q.v,2);if(r)result=(const char*)r;exec(impl_->db,"COMMIT;");return{prior==digest?ReservationStatus::Duplicate:ReservationStatus::PayloadConflict,result};}
  if(rc!=SQLITE_DONE){exec(impl_->db,"ROLLBACK;");return{ReservationStatus::StorageFailure,{}};}
  Statement i;
  if(sqlite3_prepare_v2(impl_->db,"INSERT INTO operations(operation_id,payload_digest,first_transport,route_snapshot,created_at,updated_at)VALUES(?,?,?,?,?,?)",-1,&i.v,nullptr)!=SQLITE_OK){exec(impl_->db,"ROLLBACK;");return{ReservationStatus::StorageFailure,{}};}
  sqlite3_bind_text(i.v,1,id,-1,SQLITE_TRANSIENT);sqlite3_bind_text(i.v,2,digest,-1,SQLITE_TRANSIENT);sqlite3_bind_text(i.v,3,transport,-1,SQLITE_TRANSIENT);sqlite3_bind_text(i.v,4,route,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(i.v,5,now);sqlite3_bind_int64(i.v,6,now);rc=sqlite3_step(i.v);
  if(rc!=SQLITE_DONE||exec(impl_->db,"COMMIT;")!=ESP_OK){exec(impl_->db,"ROLLBACK;");return{ReservationStatus::StorageFailure,{}};}
  return{ReservationStatus::New,{}};
}
ReservationResult DurableStorage::reserve_with_received_event(const char*id,const char*digest,const char*transport,const char*route,const char*event_id,const char*event_payload,int64_t now){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);
  if(!ready()||!id||!*id||!digest||strlen(digest)!=64||!transport||!*transport||!route||!*route||!event_id||!*event_id||!event_payload||now<=0)return{ReservationStatus::Invalid,{}};
  if(exec(impl_->db,"BEGIN IMMEDIATE;")!=ESP_OK)return{ReservationStatus::StorageFailure,{}};
  Statement q;
  if(sqlite3_prepare_v2(impl_->db,"SELECT payload_digest,result_payload FROM operations WHERE operation_id=?",-1,&q.v,nullptr)!=SQLITE_OK){exec(impl_->db,"ROLLBACK;");return{ReservationStatus::StorageFailure,{}};}
  sqlite3_bind_text(q.v,1,id,-1,SQLITE_TRANSIENT);int rc=sqlite3_step(q.v);
  if(rc==SQLITE_ROW){std::string prior=(const char*)sqlite3_column_text(q.v,0),result;const auto*r=sqlite3_column_text(q.v,1);if(r)result=(const char*)r;exec(impl_->db,"COMMIT;");return{prior==digest?ReservationStatus::Duplicate:ReservationStatus::PayloadConflict,result};}
  if(rc!=SQLITE_DONE){exec(impl_->db,"ROLLBACK;");return{ReservationStatus::StorageFailure,{}};}
  Statement operation,event;
  if(sqlite3_prepare_v2(impl_->db,"INSERT INTO operations(operation_id,payload_digest,first_transport,route_snapshot,created_at,updated_at)VALUES(?,?,?,?,?,?)",-1,&operation.v,nullptr)!=SQLITE_OK||sqlite3_prepare_v2(impl_->db,"INSERT INTO event_outbox(event_id,operation_id,kind,payload,payload_digest,created_at)VALUES(?,?,'COMMAND_RECEIVED',?,?,?)",-1,&event.v,nullptr)!=SQLITE_OK){exec(impl_->db,"ROLLBACK;");return{ReservationStatus::StorageFailure,{}};}
  sqlite3_bind_text(operation.v,1,id,-1,SQLITE_TRANSIENT);sqlite3_bind_text(operation.v,2,digest,-1,SQLITE_TRANSIENT);sqlite3_bind_text(operation.v,3,transport,-1,SQLITE_TRANSIENT);sqlite3_bind_text(operation.v,4,route,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(operation.v,5,now);sqlite3_bind_int64(operation.v,6,now);
  sqlite3_bind_text(event.v,1,event_id,-1,SQLITE_TRANSIENT);sqlite3_bind_text(event.v,2,id,-1,SQLITE_TRANSIENT);sqlite3_bind_text(event.v,3,event_payload,-1,SQLITE_TRANSIENT);sqlite3_bind_text(event.v,4,digest,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(event.v,5,now);
  if(sqlite3_step(operation.v)!=SQLITE_DONE||sqlite3_step(event.v)!=SQLITE_DONE||exec(impl_->db,"COMMIT;")!=ESP_OK){exec(impl_->db,"ROLLBACK;");return{ReservationStatus::StorageFailure,{}};}
  return{ReservationStatus::New,{}};
}
AtomicReservationResult DurableStorage::reserve_command(const char*id,const char*digest,const char*transport,const char*route,const char*event_id,const char*receipt_id,const char*canonical,const std::vector<InitialPayment>&payments,int64_t now,const std::function<std::string(const AuthorityReservation&)>&make_event,const char*execution_unp,const char*operator_code,bool claim_sale){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);
  AtomicReservationResult failure{ReservationStatus::Invalid,{},{}};
  if(!ready()||!id||!*id||!digest||strlen(digest)!=64||!transport||!*transport||!route||!*route||!event_id||!*event_id||!receipt_id||!*receipt_id||!canonical||!make_event||(operator_code&&(!authority_operator_valid(operator_code)||execution_unp))||now<=0)return failure;
  if(exec(impl_->db,"BEGIN IMMEDIATE;")!=ESP_OK){failure.status=ReservationStatus::StorageFailure;return failure;}
  auto rollback=[&](){exec(impl_->db,"ROLLBACK;");failure.status=ReservationStatus::StorageFailure;return failure;};
  Statement existing;if(sqlite3_prepare_v2(impl_->db,"SELECT payload_digest,result_payload FROM operations WHERE operation_id=?",-1,&existing.v,nullptr)!=SQLITE_OK)return rollback();sqlite3_bind_text(existing.v,1,id,-1,SQLITE_TRANSIENT);int rc=sqlite3_step(existing.v);
  if(rc==SQLITE_ROW){std::string prior=(const char*)sqlite3_column_text(existing.v,0),result;auto*r=sqlite3_column_text(existing.v,1);if(r)result=(const char*)r;if(exec(impl_->db,"COMMIT;")!=ESP_OK)return rollback();return{prior==digest?ReservationStatus::Duplicate:ReservationStatus::PayloadConflict,result,{}};}if(rc!=SQLITE_DONE)return rollback();
  Statement authority;if(sqlite3_prepare_v2(impl_->db,"SELECT prefix,next_unp,range_end,next_operation,range_start,generation FROM authority_state WHERE singleton=1",-1,&authority.v,nullptr)!=SQLITE_OK||sqlite3_step(authority.v)!=SQLITE_ROW){exec(impl_->db,"ROLLBACK;");return failure;}const int64_t unp=sqlite3_column_int64(authority.v,1),last=sqlite3_column_int64(authority.v,2),opseq=sqlite3_column_int64(authority.v,3);const bool allocate_sale=operator_code!=nullptr;std::string formatted;const std::string prefix=(const char*)sqlite3_column_text(authority.v,0);
 if(!authority_prefix_valid(prefix)||(allocate_sale&&(unp>last||!authority_unp(prefix,operator_code,unp,formatted)))||opseq<1||opseq>=std::numeric_limits<int64_t>::max()||(execution_unp&&!authority_unp_valid(execution_unp))){exec(impl_->db,"ROLLBACK;");return failure;}
 if(claim_sale&&!allocate_sale){
  if(!execution_unp||std::string(execution_unp).substr(0,8)!=prefix)return rollback();
  const int64_t assigned=authority_unp_sequence(execution_unp);if(assigned>=sqlite3_column_int64(authority.v,4)&&assigned<=last)return rollback();Statement conflict;
  if(sqlite3_prepare_v2(impl_->db,"SELECT 1 FROM authority_ranges WHERE (prefix=?1 OR (length(prefix)=14 AND substr(prefix,1,8)=?1)) AND range_start<=?2 AND range_end>=?2 LIMIT 1",-1,&conflict.v,nullptr)!=SQLITE_OK)return rollback();
  sqlite3_bind_text(conflict.v,1,prefix.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_int64(conflict.v,2,assigned);if(sqlite3_step(conflict.v)!=SQLITE_DONE)return rollback();
 }
 AuthorityReservation allocated{opseq,allocate_sale?unp:0,allocate_sale?formatted:execution_unp?execution_unp:"",prefix,sqlite3_column_int64(authority.v,4),last,sqlite3_column_int64(authority.v,5)};
  const std::string payload=make_event(allocated);if(payload.empty())return rollback();uint8_t bytes[32]{};if(mbedtls_sha256((const uint8_t*)payload.data(),payload.size(),bytes,0)!=0)return rollback();static constexpr char hex[]="0123456789abcdef";std::string event_digest(64,'0');for(size_t i=0;i<32;i++){event_digest[i*2]=hex[bytes[i]>>4];event_digest[i*2+1]=hex[bytes[i]&15];}
  Statement operation,advance,receipt,event;const bool prepared=sqlite3_prepare_v2(impl_->db,"INSERT INTO operations(operation_id,payload_digest,first_transport,route_snapshot,created_at,updated_at)VALUES(?,?,?,?,?,?)",-1,&operation.v,nullptr)==SQLITE_OK&&sqlite3_prepare_v2(impl_->db,"UPDATE authority_state SET next_unp=next_unp+CASE WHEN ?3 THEN 1 ELSE 0 END,next_operation=next_operation+1 WHERE singleton=1 AND next_unp=?1 AND next_operation=?2",-1,&advance.v,nullptr)==SQLITE_OK&&sqlite3_prepare_v2(impl_->db,"INSERT INTO receipts(receipt_id,operation_id,state,canonical_payload,created_at,updated_at)VALUES(?,?,'RESERVED',?,?,?)",-1,&receipt.v,nullptr)==SQLITE_OK&&sqlite3_prepare_v2(impl_->db,"INSERT INTO event_outbox(event_id,operation_id,kind,payload,payload_digest,created_at)VALUES(?,?,'COMMAND_RECEIVED',?,?,?)",-1,&event.v,nullptr)==SQLITE_OK;if(!prepared)return rollback();
  sqlite3_bind_text(operation.v,1,id,-1,SQLITE_TRANSIENT);sqlite3_bind_text(operation.v,2,digest,-1,SQLITE_TRANSIENT);sqlite3_bind_text(operation.v,3,transport,-1,SQLITE_TRANSIENT);sqlite3_bind_text(operation.v,4,route,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(operation.v,5,now);sqlite3_bind_int64(operation.v,6,now);sqlite3_bind_int64(advance.v,1,unp);sqlite3_bind_int64(advance.v,2,opseq);sqlite3_bind_int(advance.v,3,allocate_sale?1:0);sqlite3_bind_text(receipt.v,1,receipt_id,-1,SQLITE_TRANSIENT);sqlite3_bind_text(receipt.v,2,id,-1,SQLITE_TRANSIENT);sqlite3_bind_text(receipt.v,3,canonical,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(receipt.v,4,now);sqlite3_bind_int64(receipt.v,5,now);sqlite3_bind_text(event.v,1,event_id,-1,SQLITE_TRANSIENT);sqlite3_bind_text(event.v,2,id,-1,SQLITE_TRANSIENT);sqlite3_bind_text(event.v,3,payload.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(event.v,4,event_digest.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_int64(event.v,5,now);
  if(sqlite3_step(operation.v)!=SQLITE_DONE||sqlite3_step(advance.v)!=SQLITE_DONE||sqlite3_changes(impl_->db)!=1||sqlite3_step(receipt.v)!=SQLITE_DONE||sqlite3_step(event.v)!=SQLITE_DONE)return rollback();
  if(claim_sale||allocate_sale){Statement claim;const auto&value=allocated.unp;if(!authority_unp_valid(value)||sqlite3_prepare_v2(impl_->db,"INSERT INTO sale_unp_claims(unp,operation_id,serial,sequence)VALUES(?,?,?,?)",-1,&claim.v,nullptr)!=SQLITE_OK)return rollback();sqlite3_bind_text(claim.v,1,value.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(claim.v,2,id,-1,SQLITE_TRANSIENT);sqlite3_bind_text(claim.v,3,prefix.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_int64(claim.v,4,authority_unp_sequence(value));if(sqlite3_step(claim.v)!=SQLITE_DONE)return rollback();}
  Statement allocation;if(sqlite3_prepare_v2(impl_->db,"INSERT INTO receipt_authority(receipt_id,allocated_unp,execution_unp,operation_sequence,unp_sequence,prefix,range_start,range_end,generation)VALUES(?,?,?,?,?,?,?,?,?)",-1,&allocation.v,nullptr)!=SQLITE_OK)return rollback();
 sqlite3_bind_text(allocation.v,1,receipt_id,-1,SQLITE_TRANSIENT);sqlite3_bind_text(allocation.v,2,formatted.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(allocation.v,3,execution_unp?execution_unp:allocated.unp.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_int64(allocation.v,4,opseq);sqlite3_bind_int64(allocation.v,5,allocated.unp_sequence);sqlite3_bind_text(allocation.v,6,prefix.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_int64(allocation.v,7,allocated.range_start);sqlite3_bind_int64(allocation.v,8,last);sqlite3_bind_int64(allocation.v,9,allocated.generation);if(sqlite3_step(allocation.v)!=SQLITE_DONE)return rollback();
  for(const auto&p:payments){Statement payment;if(p.payment_id.empty()||p.amount_minor<0||sqlite3_prepare_v2(impl_->db,"INSERT INTO payments(payment_id,operation_id,receipt_id,state,amount_minor,created_at,updated_at)VALUES(?,?,?,'RESERVED',?,?,?)",-1,&payment.v,nullptr)!=SQLITE_OK)return rollback();sqlite3_bind_text(payment.v,1,p.payment_id.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(payment.v,2,id,-1,SQLITE_TRANSIENT);sqlite3_bind_text(payment.v,3,receipt_id,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(payment.v,4,p.amount_minor);sqlite3_bind_int64(payment.v,5,now);sqlite3_bind_int64(payment.v,6,now);if(sqlite3_step(payment.v)!=SQLITE_DONE)return rollback();}
  if(exec(impl_->db,"COMMIT;")!=ESP_OK)return rollback();
  return{ReservationStatus::New,{},allocated};
}
esp_err_t DurableStorage::set_operation_state(const char*id,const char*state,const char*result,int64_t now){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready()||!id||!state||now<=0)return ESP_ERR_INVALID_ARG;Statement s;if(sqlite3_prepare_v2(impl_->db,"UPDATE operations SET state=?,result_payload=?,updated_at=? WHERE operation_id=?",-1,&s.v,nullptr)!=SQLITE_OK)return ESP_FAIL;sqlite3_bind_text(s.v,1,state,-1,SQLITE_TRANSIENT);if(result)sqlite3_bind_text(s.v,2,result,-1,SQLITE_TRANSIENT);else sqlite3_bind_null(s.v,2);sqlite3_bind_int64(s.v,3,now);sqlite3_bind_text(s.v,4,id,-1,SQLITE_TRANSIENT);return sqlite3_step(s.v)==SQLITE_DONE&&sqlite3_changes(impl_->db)==1?ESP_OK:ESP_ERR_NOT_FOUND;}
esp_err_t DurableStorage::complete_operation(const char*id,const char*rid,const char*state,const char*canonical,const char*result,const std::vector<InitialPayment>&payments,const char*eid,const char*kind,const char*digest,int64_t now){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);
  if(!ready()||!id||!rid||!state||!canonical||!result||!eid||!kind||!digest||strlen(digest)!=64||now<=0)return ESP_ERR_INVALID_ARG;
  if(exec(impl_->db,"BEGIN IMMEDIATE;")!=ESP_OK)return ESP_FAIL;
  auto rollback=[&](){exec(impl_->db,"ROLLBACK;");return ESP_FAIL;};
  if(set_operation_state(id,state,result,now)!=ESP_OK||upsert_receipt(rid,id,state,canonical,now)!=ESP_OK)return rollback();
  for(const auto&p:payments)
    if(upsert_payment(p.payment_id.c_str(),id,rid,state,p.amount_minor,nullptr,nullptr,nullptr,now)!=ESP_OK)return rollback();
  if(append_event(eid,id,kind,result,digest,now)!=ESP_OK)return rollback();
  // INSERT OR IGNORE must never conceal divergent evidence under the same ID.
  Statement event;
  if(sqlite3_prepare_v2(impl_->db,"SELECT 1 FROM event_outbox WHERE event_id=? AND operation_id=? AND kind=? AND payload=? AND payload_digest=?",-1,&event.v,nullptr)!=SQLITE_OK)return rollback();
  const char*values[]={eid,id,kind,result,digest};
  for(int n=0;n<5;n++)sqlite3_bind_text(event.v,n+1,values[n],-1,SQLITE_TRANSIENT);
  if(sqlite3_step(event.v)!=SQLITE_ROW)return rollback();
  if(exec(impl_->db,"COMMIT;")!=ESP_OK)return rollback();
  return ESP_OK;
}
esp_err_t DurableStorage::operation(const char*id,OperationRecord&out)const{std::lock_guard<std::recursive_mutex> guard(impl_->mutex);out={};if(!ready()||!id||!*id)return ESP_ERR_INVALID_ARG;Statement s;if(sqlite3_prepare_v2(impl_->db,"SELECT operation_id,payload_digest,state,COALESCE(result_payload,''),updated_at FROM operations WHERE operation_id=?",-1,&s.v,nullptr)!=SQLITE_OK)return ESP_FAIL;sqlite3_bind_text(s.v,1,id,-1,SQLITE_TRANSIENT);const int rc=sqlite3_step(s.v);if(rc==SQLITE_DONE)return ESP_ERR_NOT_FOUND;if(rc!=SQLITE_ROW)return ESP_FAIL;out.operation_id=(const char*)sqlite3_column_text(s.v,0);out.payload_digest=(const char*)sqlite3_column_text(s.v,1);out.state=(const char*)sqlite3_column_text(s.v,2);out.result_payload=(const char*)sqlite3_column_text(s.v,3);out.updated_at=sqlite3_column_int64(s.v,4);return ESP_OK;}
esp_err_t DurableStorage::upsert_receipt(const char*rid,const char*oid,const char*state,const char*payload,int64_t now){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready()||!rid||!oid||!state||!payload||now<=0)return ESP_ERR_INVALID_ARG;Statement s;const char*sql="INSERT INTO receipts(receipt_id,operation_id,state,canonical_payload,created_at,updated_at)VALUES(?,?,?,?,?,?) ON CONFLICT(receipt_id)DO UPDATE SET state=excluded.state,canonical_payload=excluded.canonical_payload,updated_at=excluded.updated_at WHERE receipts.operation_id=excluded.operation_id";if(sqlite3_prepare_v2(impl_->db,sql,-1,&s.v,nullptr)!=SQLITE_OK)return ESP_FAIL;sqlite3_bind_text(s.v,1,rid,-1,SQLITE_TRANSIENT);sqlite3_bind_text(s.v,2,oid,-1,SQLITE_TRANSIENT);sqlite3_bind_text(s.v,3,state,-1,SQLITE_TRANSIENT);sqlite3_bind_text(s.v,4,payload,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(s.v,5,now);sqlite3_bind_int64(s.v,6,now);return sqlite3_step(s.v)==SQLITE_DONE&&sqlite3_changes(impl_->db)==1?ESP_OK:ESP_FAIL;}
esp_err_t DurableStorage::upsert_payment(const char*pid,const char*oid,const char*rid,const char*state,int64_t amount,const char*reference,const char*rrn,const char*auth,int64_t now){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready()||!pid||!oid||!state||amount<0||now<=0)return ESP_ERR_INVALID_ARG;Statement s;const char*sql="INSERT INTO payments(payment_id,operation_id,receipt_id,state,amount_minor,terminal_reference,rrn,auth_code,created_at,updated_at)VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(payment_id)DO UPDATE SET state=excluded.state,terminal_reference=COALESCE(excluded.terminal_reference,payments.terminal_reference),rrn=COALESCE(excluded.rrn,payments.rrn),auth_code=COALESCE(excluded.auth_code,payments.auth_code),updated_at=excluded.updated_at WHERE payments.operation_id=excluded.operation_id";if(sqlite3_prepare_v2(impl_->db,sql,-1,&s.v,nullptr)!=SQLITE_OK)return ESP_FAIL;sqlite3_bind_text(s.v,1,pid,-1,SQLITE_TRANSIENT);sqlite3_bind_text(s.v,2,oid,-1,SQLITE_TRANSIENT);if(rid)sqlite3_bind_text(s.v,3,rid,-1,SQLITE_TRANSIENT);else sqlite3_bind_null(s.v,3);sqlite3_bind_text(s.v,4,state,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(s.v,5,amount);const char*values[]={reference,rrn,auth};for(int n=0;n<3;n++)if(values[n])sqlite3_bind_text(s.v,6+n,values[n],-1,SQLITE_TRANSIENT);else sqlite3_bind_null(s.v,6+n);sqlite3_bind_int64(s.v,9,now);sqlite3_bind_int64(s.v,10,now);return sqlite3_step(s.v)==SQLITE_DONE&&sqlite3_changes(impl_->db)==1?ESP_OK:ESP_FAIL;}
esp_err_t DurableStorage::payment(const char*pid,PaymentRecord&out)const{
 std::lock_guard<std::recursive_mutex> guard(impl_->mutex);out={};if(!ready()||!pid||!*pid)return ESP_ERR_INVALID_ARG;
 Statement q;if(sqlite3_prepare_v2(impl_->db,"SELECT payment_id,operation_id,COALESCE(receipt_id,''),state,amount_minor,COALESCE(terminal_reference,''),COALESCE(rrn,''),COALESCE(auth_code,''),COALESCE((SELECT reversal_reference FROM payment_saga_members m WHERE m.payment_id=payments.payment_id),''),COALESCE((SELECT reversal_rrn FROM payment_saga_members m WHERE m.payment_id=payments.payment_id),''),COALESCE((SELECT reversal_auth_code FROM payment_saga_members m WHERE m.payment_id=payments.payment_id),'') FROM payments WHERE payment_id=?",-1,&q.v,nullptr)!=SQLITE_OK)return ESP_FAIL;
 sqlite3_bind_text(q.v,1,pid,-1,SQLITE_TRANSIENT);int rc=sqlite3_step(q.v);if(rc==SQLITE_DONE)return ESP_ERR_NOT_FOUND;if(rc!=SQLITE_ROW)return ESP_FAIL;
 out.payment_id=(const char*)sqlite3_column_text(q.v,0);out.operation_id=(const char*)sqlite3_column_text(q.v,1);out.receipt_id=(const char*)sqlite3_column_text(q.v,2);out.state=(const char*)sqlite3_column_text(q.v,3);out.amount_minor=sqlite3_column_int64(q.v,4);out.terminal_reference=(const char*)sqlite3_column_text(q.v,5);out.rrn=(const char*)sqlite3_column_text(q.v,6);out.auth_code=(const char*)sqlite3_column_text(q.v,7);out.reversal_reference=(const char*)sqlite3_column_text(q.v,8);out.reversal_rrn=(const char*)sqlite3_column_text(q.v,9);out.reversal_auth_code=(const char*)sqlite3_column_text(q.v,10);return ESP_OK;
}
esp_err_t DurableStorage::payment_saga_phase(const char*oid,std::string&out)const{
 std::lock_guard<std::recursive_mutex> guard(impl_->mutex);out.clear();if(!ready()||!oid||!*oid)return ESP_ERR_INVALID_ARG;
 Statement q;if(sqlite3_prepare_v2(impl_->db,"SELECT phase FROM payment_saga WHERE operation_id=?",-1,&q.v,nullptr)!=SQLITE_OK)return ESP_FAIL;sqlite3_bind_text(q.v,1,oid,-1,SQLITE_TRANSIENT);int rc=sqlite3_step(q.v);if(rc==SQLITE_DONE)return ESP_ERR_NOT_FOUND;if(rc!=SQLITE_ROW)return ESP_FAIL;out=(const char*)sqlite3_column_text(q.v,0);return ESP_OK;
}
esp_err_t DurableStorage::payment_saga_snapshot(const char*oid,const char*rid,PaymentSagaSnapshot&out)const{
 std::lock_guard<std::recursive_mutex> guard(impl_->mutex);out={};if(!ready()||!oid||!*oid||!rid||!*rid)return ESP_ERR_INVALID_ARG;
 // A read transaction is required as another SQLite connection can update rows.
 if(exec(impl_->db,"BEGIN;")!=ESP_OK)return ESP_FAIL;
 auto fail=[&](esp_err_t e){exec(impl_->db,"ROLLBACK;");out={};return e;};
 auto rc=payment_saga_phase(oid,out.phase);if(rc!=ESP_OK)return fail(rc);
 OperationRecord owner;if(operation(oid,owner)!=ESP_OK||(owner.state!="PAYMENT_SAGA_PENDING"&&owner.state!="COMPENSATION_REQUIRED"&&owner.state!="RECOVERY_REQUIRED"))return fail(ESP_ERR_INVALID_STATE);
 Statement q;if(sqlite3_prepare_v2(impl_->db,"SELECT payment_id FROM payment_saga_members WHERE operation_id=? ORDER BY rowid",-1,&q.v,nullptr)!=SQLITE_OK)return fail(ESP_FAIL);
 sqlite3_bind_text(q.v,1,oid,-1,SQLITE_TRANSIENT);int step;
 while((step=sqlite3_step(q.v))==SQLITE_ROW){PaymentRecord p;if(payment((const char*)sqlite3_column_text(q.v,0),p)!=ESP_OK||p.operation_id!=oid||p.receipt_id!=rid||p.amount_minor<=0)return fail(ESP_FAIL);out.payments.push_back(p);}
 if(step!=SQLITE_DONE||out.payments.empty()||exec(impl_->db,"COMMIT;")!=ESP_OK)return fail(ESP_FAIL);
 return ESP_OK;
}
esp_err_t DurableStorage::reconcile_payment(const PaymentRecord&expected,const PaymentRecord&observed,const char*next,int64_t now,const PaymentReconciliationEvidence*evidence){
 std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready()||!next||now<=0)return ESP_ERR_INVALID_ARG;
 const auto&from=expected.state;const std::string to=next;
 bool reversal=from=="REVERSAL_SENDING"||from=="REVERSAL_UNKNOWN";
 if(!((from=="CARD_SENDING"||from=="CARD_UNKNOWN")&&(to=="CARD_APPROVED"||to=="CARD_DECLINED"||to=="CARD_NOT_SENT"))&&!(reversal&&(to=="REVERSED"||to=="REVERSAL_FAILED")))return ESP_ERR_INVALID_ARG;
 if(expected.payment_id!=observed.payment_id||expected.operation_id!=observed.operation_id||expected.receipt_id!=observed.receipt_id||expected.amount_minor!=observed.amount_minor)return ESP_ERR_INVALID_ARG;
 if(exec(impl_->db,"BEGIN IMMEDIATE;")!=ESP_OK)return ESP_FAIL;
 auto fail=[&](){exec(impl_->db,"ROLLBACK;");return ESP_FAIL;};
 OperationRecord op;if(operation(expected.operation_id.c_str(),op)!=ESP_OK||(op.state!="RECOVERY_REQUIRED"&&op.state!="PAYMENT_SAGA_PENDING"&&op.state!="COMPENSATION_REQUIRED"))return fail();
 PaymentSagaSnapshot snapshot;if(payment_saga_phase(expected.operation_id.c_str(),snapshot.phase)!=ESP_OK)return fail();
 if((!reversal&&snapshot.phase!="PAYMENTS")||(reversal&&snapshot.phase!="COMPENSATING"))return fail();
 PaymentRecord current;if(payment(expected.payment_id.c_str(),current)!=ESP_OK||current.operation_id!=expected.operation_id||current.receipt_id!=expected.receipt_id||current.amount_minor!=expected.amount_minor||current.state!=expected.state||current.terminal_reference!=expected.terminal_reference||current.rrn!=expected.rrn||current.auth_code!=expected.auth_code||current.reversal_reference!=expected.reversal_reference||current.reversal_rrn!=expected.reversal_rrn||current.reversal_auth_code!=expected.reversal_auth_code)return fail();
 // Existing purchase identity is immutable even if a terminal query contradicts it.
 if((!current.terminal_reference.empty()&&observed.terminal_reference!=current.terminal_reference)||(!current.rrn.empty()&&observed.rrn!=current.rrn)||(!current.auth_code.empty()&&observed.auth_code!=current.auth_code))return fail();
 Statement q;if(sqlite3_prepare_v2(impl_->db,"UPDATE payments SET state=?,terminal_reference=?,rrn=?,auth_code=?,updated_at=? WHERE payment_id=? AND EXISTS(SELECT 1 FROM payment_saga_members m WHERE m.payment_id=payments.payment_id AND m.operation_id=payments.operation_id)",-1,&q.v,nullptr)!=SQLITE_OK)return fail();
 sqlite3_bind_text(q.v,1,next,-1,SQLITE_TRANSIENT);const std::string*refs[]={&observed.terminal_reference,&observed.rrn,&observed.auth_code};for(int n=0;n<3;n++)sqlite3_bind_text(q.v,n+2,refs[n]->c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_int64(q.v,5,now);sqlite3_bind_text(q.v,6,expected.payment_id.c_str(),-1,SQLITE_TRANSIENT);if(sqlite3_step(q.v)!=SQLITE_DONE||sqlite3_changes(impl_->db)!=1)return fail();
 if(reversal){Statement m;if(sqlite3_prepare_v2(impl_->db,"UPDATE payment_saga_members SET reversal_reference=?,reversal_rrn=?,reversal_auth_code=? WHERE operation_id=? AND payment_id=?",-1,&m.v,nullptr)!=SQLITE_OK)return fail();const std::string*refs[]={&observed.reversal_reference,&observed.reversal_rrn,&observed.reversal_auth_code};for(int n=0;n<3;n++)sqlite3_bind_text(m.v,n+1,refs[n]->c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(m.v,4,expected.operation_id.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(m.v,5,expected.payment_id.c_str(),-1,SQLITE_TRANSIENT);if(sqlite3_step(m.v)!=SQLITE_DONE||sqlite3_changes(impl_->db)!=1)return fail();}
 if(evidence){
  const auto&e=*evidence;if(to=="CARD_APPROVED"&&(observed.terminal_reference.empty()||observed.rrn.empty()||observed.auth_code.empty()))return fail();if(to=="REVERSED"&&(observed.reversal_reference.empty()||observed.reversal_rrn.empty()||observed.reversal_auth_code.empty()))return fail();if(e.reconciliation_id.empty()||e.reconciliation_id.size()>128||e.terminal_id.empty()||e.terminal_id.size()>128||e.actor_id.empty()||e.actor_id.size()>kAuthorityActorMaxBytes||e.evidence_reference.empty()||e.evidence_reference.size()>1024||!hex_digest(e.evidence_sha256)||!hex_digest(e.authorization_sha256))return fail();
  Statement guardrow;if(sqlite3_prepare_v2(impl_->db,"SELECT 1 FROM terminal_payment_guard WHERE terminal_id=? AND operation_id=?",-1,&guardrow.v,nullptr)!=SQLITE_OK)return fail();sqlite3_bind_text(guardrow.v,1,e.terminal_id.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(guardrow.v,2,expected.operation_id.c_str(),-1,SQLITE_TRANSIENT);if(sqlite3_step(guardrow.v)!=SQLITE_ROW)return fail();
  Statement audit;const char*values[]={e.reconciliation_id.c_str(),e.terminal_id.c_str(),expected.payment_id.c_str(),expected.operation_id.c_str(),e.actor_id.c_str(),e.evidence_reference.c_str(),e.evidence_sha256.c_str(),e.authorization_sha256.c_str(),expected.state.c_str(),next};
  // Explicit column list prevents a schema extension from changing evidence order.
  if(sqlite3_prepare_v2(impl_->db,"INSERT INTO payment_reconciliations(reconciliation_id,terminal_id,payment_id,operation_id,actor_id,evidence_reference,evidence_sha256,authorization_sha256,old_state,new_state,created_at)VALUES(?,?,?,?,?,?,?,?,?,?,?)",-1,&audit.v,nullptr)!=SQLITE_OK)return fail();
  for(int n=0;n<10;++n)sqlite3_bind_text(audit.v,n+1,values[n],-1,SQLITE_TRANSIENT);
  sqlite3_bind_int64(audit.v,11,now);if(sqlite3_step(audit.v)!=SQLITE_DONE)return fail();
  auto*payload=cJSON_CreateObject();if(!payload)return fail();const char*names[]={"reconciliation_id","terminal_id","payment_id","operation_id","actor_id","evidence_reference","evidence_sha256","authorization_sha256","old_state","new_state"};bool ok=true;for(int n=0;n<10;++n)ok=ok&&cJSON_AddStringToObject(payload,names[n],values[n]);ok=ok&&cJSON_AddStringToObject(payload,"amount_minor",std::to_string(observed.amount_minor).c_str())&&cJSON_AddStringToObject(payload,"terminal_reference",observed.terminal_reference.c_str())&&cJSON_AddStringToObject(payload,"rrn",observed.rrn.c_str())&&cJSON_AddStringToObject(payload,"auth_code",observed.auth_code.c_str())&&cJSON_AddStringToObject(payload,"reversal_reference",observed.reversal_reference.c_str())&&cJSON_AddStringToObject(payload,"reversal_rrn",observed.reversal_rrn.c_str())&&cJSON_AddStringToObject(payload,"reversal_auth_code",observed.reversal_auth_code.c_str());char*body=ok?cJSON_PrintUnformatted(payload):nullptr;cJSON_Delete(payload);if(!body)return fail();unsigned char hash[32];if(mbedtls_sha256((const unsigned char*)body,strlen(body),hash,0)!=0){cJSON_free(body);return fail();}char digest[65];const char*digits="0123456789abcdef";for(int n=0;n<32;++n){digest[2*n]=digits[hash[n]>>4];digest[2*n+1]=digits[hash[n]&15];}digest[64]=0;// Persist the administrative command result in this same transaction. The
  // original financial operation remains unresolved; this result certifies only
  // successful application of the signed bank-decision CAS.
  Statement adminop;
  if(e.reconciliation_id==expected.operation_id||sqlite3_prepare_v2(impl_->db,"INSERT INTO operations(operation_id,payload_digest,first_transport,route_snapshot,state,result_payload,created_at,updated_at)VALUES(?,?,'ADMIN','{}','COMMITTED',?,?,?)",-1,&adminop.v,nullptr)!=SQLITE_OK){cJSON_free(body);return fail();}
  sqlite3_bind_text(adminop.v,1,e.reconciliation_id.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(adminop.v,2,e.authorization_sha256.c_str(),-1,SQLITE_TRANSIENT);auto*adminresult=cJSON_Parse(body);if(!adminresult){cJSON_free(body);return fail();}cJSON_SetValuestring(cJSON_GetObjectItemCaseSensitive(adminresult,"operation_id"),e.reconciliation_id.c_str());if(!cJSON_AddStringToObject(adminresult,"original_operation_id",expected.operation_id.c_str())||!cJSON_AddStringToObject(adminresult,"state","COMMITTED")||!cJSON_AddStringToObject(adminresult,"effect","BANK_DECISION_RECORDED_FISCAL_UNRESOLVED")){cJSON_Delete(adminresult);cJSON_free(body);return fail();}char*adminbody=cJSON_PrintUnformatted(adminresult);cJSON_Delete(adminresult);if(!adminbody){cJSON_free(body);return fail();}sqlite3_bind_text(adminop.v,3,adminbody,-1,SQLITE_TRANSIENT);cJSON_free(adminbody);sqlite3_bind_int64(adminop.v,4,now);sqlite3_bind_int64(adminop.v,5,now);if(sqlite3_step(adminop.v)!=SQLITE_DONE){cJSON_free(body);return fail();}
  const auto result=append_event((e.reconciliation_id+"-reconciled").c_str(),expected.operation_id.c_str(),"PAYMENT_RECONCILED",body,digest,now);if(result!=ESP_OK){cJSON_free(body);return fail();}Statement published;if(sqlite3_prepare_v2(impl_->db,"SELECT 1 FROM event_outbox WHERE event_id=? AND operation_id=? AND kind='PAYMENT_RECONCILED' AND payload=? AND payload_digest=?",-1,&published.v,nullptr)!=SQLITE_OK){cJSON_free(body);return fail();}sqlite3_bind_text(published.v,1,(e.reconciliation_id+"-reconciled").c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(published.v,2,expected.operation_id.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(published.v,3,body,-1,SQLITE_TRANSIENT);sqlite3_bind_text(published.v,4,digest,-1,SQLITE_TRANSIENT);cJSON_free(body);if(sqlite3_step(published.v)!=SQLITE_ROW)return fail();
 }
 if(set_operation_state(expected.operation_id.c_str(),"RECOVERY_REQUIRED",nullptr,now)!=ESP_OK||exec(impl_->db,"COMMIT;")!=ESP_OK)return fail();
 return ESP_OK;
}

esp_err_t DurableStorage::begin_payment_saga(const char*oid,const char*rid,const std::vector<InitialPayment>&cards,int64_t now,const char*terminal){
 std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready()||!oid||!*oid||!rid||!*rid||cards.empty()||now<=0)return ESP_ERR_INVALID_ARG;
 if(exec(impl_->db,"BEGIN IMMEDIATE;")!=ESP_OK)return ESP_FAIL;
 auto rollback=[&](){exec(impl_->db,"ROLLBACK;");return ESP_FAIL;};
 OperationRecord operation;if(this->operation(oid,operation)!=ESP_OK||operation.state!="EXECUTING")return rollback();
 if(terminal){const auto claimed=claim_terminal(impl_->db,terminal,oid,now);if(claimed!=ESP_OK){exec(impl_->db,"ROLLBACK;");return claimed;}}
 std::set<std::string> ids;
 for(const auto&card:cards){PaymentRecord p;if(!ids.insert(card.payment_id).second)return rollback();if(payment(card.payment_id.c_str(),p)!=ESP_OK||p.operation_id!=oid||p.receipt_id!=rid||p.amount_minor!=card.amount_minor||p.state!="RESERVED")return rollback();}
 Statement insert;if(sqlite3_prepare_v2(impl_->db,"INSERT INTO payment_saga(operation_id,phase,updated_at)VALUES(?,'PAYMENTS',?)",-1,&insert.v,nullptr)!=SQLITE_OK)return rollback();sqlite3_bind_text(insert.v,1,oid,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(insert.v,2,now);
 if(sqlite3_step(insert.v)!=SQLITE_DONE)return rollback();
 for(const auto&card:cards){Statement member;if(sqlite3_prepare_v2(impl_->db,"INSERT INTO payment_saga_members(operation_id,payment_id)VALUES(?,?)",-1,&member.v,nullptr)!=SQLITE_OK)return rollback();sqlite3_bind_text(member.v,1,oid,-1,SQLITE_TRANSIENT);sqlite3_bind_text(member.v,2,card.payment_id.c_str(),-1,SQLITE_TRANSIENT);if(sqlite3_step(member.v)!=SQLITE_DONE)return rollback();}
 if(set_operation_state(oid,"PAYMENT_SAGA_PENDING",nullptr,now)!=ESP_OK||exec(impl_->db,"COMMIT;")!=ESP_OK)return rollback();
 return ESP_OK;
}
esp_err_t DurableStorage::checkpoint_payment(const PaymentRecord&p,const char*expected,const char*next,const char*phase,int64_t now){
 std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready()||!expected||!next||!phase||p.amount_minor<=0||now<=0)return ESP_ERR_INVALID_ARG;
 const std::string from=expected,to=next,target=phase;
 const bool purchase=(from=="RESERVED"&&to=="CARD_SENDING")||(from=="CARD_SENDING"&&(to=="CARD_APPROVED"||to=="CARD_DECLINED"||to=="CARD_NOT_SENT"||to=="CARD_UNKNOWN"));
 const bool reversal=(from=="CARD_APPROVED"&&to=="REVERSAL_SENDING")||(from=="REVERSAL_SENDING"&&(to=="REVERSED"||to=="REVERSAL_UNKNOWN"||to=="REVERSAL_FAILED"));
 if((!purchase&&!reversal)||(purchase&&target!="PAYMENTS")||(reversal&&target!="COMPENSATING"))return ESP_ERR_INVALID_ARG;
 if(exec(impl_->db,"BEGIN IMMEDIATE;")!=ESP_OK)return ESP_FAIL;
 auto rollback=[&](){exec(impl_->db,"ROLLBACK;");return ESP_FAIL;};
 OperationRecord operation;if(this->operation(p.operation_id.c_str(),operation)!=ESP_OK||(operation.state!="PAYMENT_SAGA_PENDING"&&operation.state!="COMPENSATION_REQUIRED"&&!(reversal&&operation.state=="RECOVERY_REQUIRED")))return rollback();
 std::string current;if(payment_saga_phase(p.operation_id.c_str(),current)!=ESP_OK||(purchase&&current!="PAYMENTS")||(reversal&&current!="PAYMENTS"&&current!="COMPENSATING"&&current!="FISCAL_SENDING"&&current!="COMPENSATION_REQUIRED"))return rollback();
 Statement update;if(sqlite3_prepare_v2(impl_->db,"UPDATE payments SET state=?,terminal_reference=COALESCE(?,terminal_reference),rrn=COALESCE(?,rrn),auth_code=COALESCE(?,auth_code),updated_at=? WHERE payment_id=? AND operation_id=? AND receipt_id=? AND amount_minor=? AND state=? AND EXISTS(SELECT 1 FROM payment_saga_members m WHERE m.operation_id=payments.operation_id AND m.payment_id=payments.payment_id)",-1,&update.v,nullptr)!=SQLITE_OK)return rollback();
 sqlite3_bind_text(update.v,1,next,-1,SQLITE_TRANSIENT);const std::string*refs[]={&p.terminal_reference,&p.rrn,&p.auth_code};for(int n=0;n<3;n++){if(refs[n]->empty())sqlite3_bind_null(update.v,n+2);else sqlite3_bind_text(update.v,n+2,refs[n]->c_str(),-1,SQLITE_TRANSIENT);}sqlite3_bind_int64(update.v,5,now);sqlite3_bind_text(update.v,6,p.payment_id.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(update.v,7,p.operation_id.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(update.v,8,p.receipt_id.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_int64(update.v,9,p.amount_minor);sqlite3_bind_text(update.v,10,expected,-1,SQLITE_TRANSIENT);
 if(sqlite3_step(update.v)!=SQLITE_DONE||sqlite3_changes(impl_->db)!=1)return rollback();
 if(from=="REVERSAL_SENDING"){
  Statement evidence;if(sqlite3_prepare_v2(impl_->db,"UPDATE payment_saga_members SET reversal_reference=?,reversal_rrn=?,reversal_auth_code=? WHERE operation_id=? AND payment_id=?",-1,&evidence.v,nullptr)!=SQLITE_OK)return rollback();const std::string*refs[]={&p.reversal_reference,&p.reversal_rrn,&p.reversal_auth_code};for(int n=0;n<3;n++){if(refs[n]->empty())sqlite3_bind_null(evidence.v,n+1);else sqlite3_bind_text(evidence.v,n+1,refs[n]->c_str(),-1,SQLITE_TRANSIENT);}sqlite3_bind_text(evidence.v,4,p.operation_id.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(evidence.v,5,p.payment_id.c_str(),-1,SQLITE_TRANSIENT);if(sqlite3_step(evidence.v)!=SQLITE_DONE||sqlite3_changes(impl_->db)!=1)return rollback();
 }
 Statement saga;if(sqlite3_prepare_v2(impl_->db,"UPDATE payment_saga SET phase=?,updated_at=? WHERE operation_id=?",-1,&saga.v,nullptr)!=SQLITE_OK)return rollback();sqlite3_bind_text(saga.v,1,phase,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(saga.v,2,now);sqlite3_bind_text(saga.v,3,p.operation_id.c_str(),-1,SQLITE_TRANSIENT);
 if(sqlite3_step(saga.v)!=SQLITE_DONE||sqlite3_changes(impl_->db)!=1||set_operation_state(p.operation_id.c_str(),reversal?"COMPENSATION_REQUIRED":"PAYMENT_SAGA_PENDING",nullptr,now)!=ESP_OK||exec(impl_->db,"COMMIT;")!=ESP_OK)return rollback();
 return ESP_OK;
}
esp_err_t DurableStorage::checkpoint_saga_phase(const char*oid,const char*expected,const char*next,int64_t now){
 std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready()||!oid||!expected||!next||now<=0)return ESP_ERR_INVALID_ARG;
 const std::string from=expected,to=next;if(!((from=="PAYMENTS"&&to=="FISCAL_SENDING")||(from=="COMPENSATING"&&to=="COMPENSATED")||((from=="PAYMENTS"||from=="FISCAL_SENDING")&&to=="COMPENSATION_REQUIRED")))return ESP_ERR_INVALID_ARG;
 if(exec(impl_->db,"BEGIN IMMEDIATE;")!=ESP_OK)return ESP_FAIL;
 auto rollback=[&](){exec(impl_->db,"ROLLBACK;");return ESP_FAIL;};
 OperationRecord operation;if(this->operation(oid,operation)!=ESP_OK||(operation.state!="PAYMENT_SAGA_PENDING"&&operation.state!="COMPENSATION_REQUIRED"&&!(to=="COMPENSATED"&&operation.state=="RECOVERY_REQUIRED")))return rollback();
 if(to=="FISCAL_SENDING"||to=="COMPENSATED"){
  const char*query=to=="FISCAL_SENDING"?"SELECT count(*) FROM payment_saga_members m JOIN payments p ON p.payment_id=m.payment_id WHERE m.operation_id=? AND p.state!='CARD_APPROVED'":"SELECT count(*) FROM payment_saga_members m JOIN payments p ON p.payment_id=m.payment_id WHERE m.operation_id=? AND p.state NOT IN('REVERSED','CARD_DECLINED','CARD_NOT_SENT','RESERVED')";
  Statement verify;if(sqlite3_prepare_v2(impl_->db,query,-1,&verify.v,nullptr)!=SQLITE_OK)return rollback();sqlite3_bind_text(verify.v,1,oid,-1,SQLITE_TRANSIENT);if(sqlite3_step(verify.v)!=SQLITE_ROW||sqlite3_column_int64(verify.v,0)!=0)return rollback();
 }
 Statement q;if(sqlite3_prepare_v2(impl_->db,"UPDATE payment_saga SET phase=?,updated_at=? WHERE operation_id=? AND phase=?",-1,&q.v,nullptr)!=SQLITE_OK)return rollback();sqlite3_bind_text(q.v,1,next,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(q.v,2,now);sqlite3_bind_text(q.v,3,oid,-1,SQLITE_TRANSIENT);sqlite3_bind_text(q.v,4,expected,-1,SQLITE_TRANSIENT);
 if(sqlite3_step(q.v)!=SQLITE_DONE||sqlite3_changes(impl_->db)!=1||set_operation_state(oid,to=="COMPENSATION_REQUIRED"?"COMPENSATION_REQUIRED":"PAYMENT_SAGA_PENDING",nullptr,now)!=ESP_OK||exec(impl_->db,"COMMIT;")!=ESP_OK)return rollback();
 return ESP_OK;
}
esp_err_t DurableStorage::append_event(const char*eid,const char*oid,const char*kind,const char*payload,const char*digest,int64_t now){
 std::lock_guard<std::recursive_mutex> guard(impl_->mutex);
 if(!ready()||!eid||!*eid||!oid||!kind||!payload||!digest||strlen(digest)!=64||now<=0)return ESP_ERR_INVALID_ARG;
 Statement existing;
 if(sqlite3_prepare_v2(impl_->db,"SELECT operation_id,kind,payload,payload_digest,created_at FROM event_outbox WHERE event_id=?",-1,&existing.v,nullptr)!=SQLITE_OK)return ESP_FAIL;
 sqlite3_bind_text(existing.v,1,eid,-1,SQLITE_TRANSIENT);
 const int found=sqlite3_step(existing.v);
 if(found==SQLITE_ROW){
  const char*expected[]={oid,kind,payload,digest};
  for(int n=0;n<4;n++)if(strcmp((const char*)sqlite3_column_text(existing.v,n),expected[n])!=0)return ESP_ERR_INVALID_STATE;
  return sqlite3_column_int64(existing.v,4)==now?ESP_OK:ESP_ERR_INVALID_STATE;
 }
 if(found!=SQLITE_DONE)return ESP_FAIL;
 Statement insert;
 if(sqlite3_prepare_v2(impl_->db,"INSERT INTO event_outbox(event_id,operation_id,kind,payload,payload_digest,created_at)VALUES(?,?,?,?,?,?)",-1,&insert.v,nullptr)!=SQLITE_OK)return ESP_FAIL;
 const char*values[]={eid,oid,kind,payload,digest};
 for(int n=0;n<5;n++)sqlite3_bind_text(insert.v,n+1,values[n],-1,SQLITE_TRANSIENT);
 sqlite3_bind_int64(insert.v,6,now);
 return sqlite3_step(insert.v)==SQLITE_DONE?ESP_OK:ESP_FAIL;
}
esp_err_t DurableStorage::pending(size_t limit,const std::function<bool(const PendingEvent&)>&visitor){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready()||!visitor||!limit)return ESP_ERR_INVALID_ARG;Statement s;if(sqlite3_prepare_v2(impl_->db,"SELECT sequence,event_id,operation_id,kind,payload,payload_digest,created_at,attempt_count FROM event_outbox WHERE acknowledged_at IS NULL ORDER BY sequence LIMIT ?",-1,&s.v,nullptr)!=SQLITE_OK)return ESP_FAIL;sqlite3_bind_int64(s.v,1,limit);int rc;while((rc=sqlite3_step(s.v))==SQLITE_ROW){PendingEvent e{sqlite3_column_int64(s.v,0),(const char*)sqlite3_column_text(s.v,1),(const char*)sqlite3_column_text(s.v,2),(const char*)sqlite3_column_text(s.v,3),(const char*)sqlite3_column_text(s.v,4),(const char*)sqlite3_column_text(s.v,5),sqlite3_column_int64(s.v,6),(uint32_t)sqlite3_column_int64(s.v,7)};if(!visitor(e))break;}return rc==SQLITE_DONE||rc==SQLITE_ROW?ESP_OK:ESP_FAIL;}
esp_err_t DurableStorage::record_publish_attempt(const char*eid,const char*error,int64_t now){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready()||!eid||now<=0)return ESP_ERR_INVALID_ARG;Statement s;if(sqlite3_prepare_v2(impl_->db,"UPDATE event_outbox SET attempt_count=attempt_count+1,last_attempt_at=?,last_error=? WHERE event_id=? AND acknowledged_at IS NULL",-1,&s.v,nullptr)!=SQLITE_OK)return ESP_FAIL;sqlite3_bind_int64(s.v,1,now);if(error)sqlite3_bind_text(s.v,2,error,-1,SQLITE_TRANSIENT);else sqlite3_bind_null(s.v,2);sqlite3_bind_text(s.v,3,eid,-1,SQLITE_TRANSIENT);return sqlite3_step(s.v)==SQLITE_DONE?ESP_OK:ESP_FAIL;}
esp_err_t DurableStorage::frozen_sync_batch(FrozenSyncBatch&out)const{std::lock_guard<std::recursive_mutex> guard(impl_->mutex);
 out={};if(!ready())return ESP_ERR_INVALID_STATE;Statement q;
 if(sqlite3_prepare_v2(impl_->db,"SELECT batch_id,body,first_sequence,last_sequence,last_event_hash,previous_hash FROM frozen_sync_batch WHERE singleton=1",-1,&q.v,nullptr)!=SQLITE_OK)return ESP_FAIL;
 int rc=sqlite3_step(q.v);if(rc==SQLITE_DONE)return ESP_ERR_NOT_FOUND;if(rc!=SQLITE_ROW)return ESP_FAIL;
 out.batch_id=(const char*)sqlite3_column_text(q.v,0);out.body=(const char*)sqlite3_column_text(q.v,1);out.first_sequence=sqlite3_column_int64(q.v,2);out.last_sequence=sqlite3_column_int64(q.v,3);out.last_event_hash=(const char*)sqlite3_column_text(q.v,4);out.previous_hash=(const char*)sqlite3_column_text(q.v,5);return ESP_OK;
}
esp_err_t DurableStorage::freeze_sync_batch(const FrozenSyncBatch&batch,const std::vector<PendingEvent>&events){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);
 if(!ready()||batch.batch_id.empty()||batch.body.empty()||batch.last_event_hash.size()!=64||events.empty()||batch.first_sequence!=events.front().sequence||batch.last_sequence!=events.back().sequence)return ESP_ERR_INVALID_ARG;
 if(exec(impl_->db,"BEGIN IMMEDIATE;")!=ESP_OK)return ESP_FAIL;
 auto rollback=[&](){exec(impl_->db,"ROLLBACK;");return ESP_FAIL;};
 FrozenSyncBatch existing;const auto found=frozen_sync_batch(existing);if(found!=ESP_ERR_NOT_FOUND)return rollback();
 if(batch.first_sequence!=acknowledged_cursor()+1||batch.previous_hash!=acknowledged_event_hash())return rollback();
 if(exec(impl_->db,"DELETE FROM frozen_sync_members;")!=ESP_OK)return rollback();
 int64_t expected=batch.first_sequence;
 for(const auto&e:events){
  if(e.sequence!=expected++)return rollback();
  Statement member;
  if(sqlite3_prepare_v2(impl_->db,"INSERT INTO frozen_sync_members SELECT sequence,event_id,operation_id,kind,payload,payload_digest,created_at FROM event_outbox WHERE sequence=? AND event_id=? AND operation_id=? AND kind=? AND payload=? AND payload_digest=? AND created_at=? AND acknowledged_at IS NULL",-1,&member.v,nullptr)!=SQLITE_OK)return rollback();
  sqlite3_bind_int64(member.v,1,e.sequence);const char*values[]={e.event_id.c_str(),e.operation_id.c_str(),e.kind.c_str(),e.payload.c_str(),e.payload_digest.c_str()};for(int n=0;n<5;n++)sqlite3_bind_text(member.v,n+2,values[n],-1,SQLITE_TRANSIENT);sqlite3_bind_int64(member.v,7,e.created_at);
  if(sqlite3_step(member.v)!=SQLITE_DONE||sqlite3_changes(impl_->db)!=1)return rollback();
 }
 Statement insert;
 if(sqlite3_prepare_v2(impl_->db,"INSERT INTO frozen_sync_batch VALUES(1,?,?,?,?,?,?)",-1,&insert.v,nullptr)!=SQLITE_OK)return rollback();
 sqlite3_bind_text(insert.v,1,batch.batch_id.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(insert.v,2,batch.body.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_int64(insert.v,3,batch.first_sequence);sqlite3_bind_int64(insert.v,4,batch.last_sequence);sqlite3_bind_text(insert.v,5,batch.last_event_hash.c_str(),-1,SQLITE_TRANSIENT);sqlite3_bind_text(insert.v,6,batch.previous_hash.c_str(),-1,SQLITE_TRANSIENT);
 if(sqlite3_step(insert.v)!=SQLITE_DONE||exec(impl_->db,"COMMIT;")!=ESP_OK)return rollback();
 return ESP_OK;
}
esp_err_t DurableStorage::acknowledge_through(int64_t seq,const char*ack,const char*event_hash,int64_t now){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);
 if(!ready()||seq<1||!ack||!*ack||!event_hash||strlen(event_hash)!=64||now<=0)return ESP_ERR_INVALID_ARG;
 if(exec(impl_->db,"BEGIN IMMEDIATE;")!=ESP_OK)return ESP_FAIL;
 auto rollback=[&](){exec(impl_->db,"ROLLBACK;");return ESP_FAIL;};
 const auto cursor=acknowledged_cursor();const auto cursor_hash=acknowledged_event_hash();
 // An old signed ACK cannot overwrite the current hash or acknowledge a new batch.
 if(seq==cursor&&cursor_hash==event_hash){if(exec(impl_->db,"COMMIT;")!=ESP_OK)return rollback();
 return ESP_OK;}
 FrozenSyncBatch batch;if(frozen_sync_batch(batch)!=ESP_OK||seq!=batch.last_sequence||batch.last_event_hash!=event_hash||batch.first_sequence!=cursor+1||batch.previous_hash!=cursor_hash)return rollback();
 Statement verify;
 if(sqlite3_prepare_v2(impl_->db,"SELECT COUNT(*) FROM frozen_sync_members m JOIN event_outbox e ON e.sequence=m.sequence AND e.event_id=m.event_id AND e.operation_id=m.operation_id AND e.kind=m.kind AND e.payload=m.payload AND e.payload_digest=m.payload_digest AND e.created_at=m.created_at AND e.acknowledged_at IS NULL WHERE m.sequence BETWEEN ? AND ?",-1,&verify.v,nullptr)!=SQLITE_OK)return rollback();
 sqlite3_bind_int64(verify.v,1,batch.first_sequence);sqlite3_bind_int64(verify.v,2,batch.last_sequence);
 if(sqlite3_step(verify.v)!=SQLITE_ROW||sqlite3_column_int64(verify.v,0)!=batch.last_sequence-batch.first_sequence+1)return rollback();
 Statement update,advance;
 if(sqlite3_prepare_v2(impl_->db,"UPDATE event_outbox SET acknowledged_at=?,ack_id=?,last_error=NULL WHERE sequence BETWEEN ? AND ? AND acknowledged_at IS NULL",-1,&update.v,nullptr)!=SQLITE_OK)return rollback();
 sqlite3_bind_int64(update.v,1,now);sqlite3_bind_text(update.v,2,ack,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(update.v,3,batch.first_sequence);sqlite3_bind_int64(update.v,4,batch.last_sequence);
 if(sqlite3_step(update.v)!=SQLITE_DONE||sqlite3_changes(impl_->db)!=batch.last_sequence-batch.first_sequence+1)return rollback();
 if(sqlite3_prepare_v2(impl_->db,"UPDATE sync_cursor SET acknowledged_sequence=?,ack_id=?,committed_event_hash=?,updated_at=? WHERE singleton=1",-1,&advance.v,nullptr)!=SQLITE_OK)return rollback();
 sqlite3_bind_int64(advance.v,1,seq);sqlite3_bind_text(advance.v,2,ack,-1,SQLITE_TRANSIENT);sqlite3_bind_text(advance.v,3,event_hash,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(advance.v,4,now);
 if(sqlite3_step(advance.v)!=SQLITE_DONE||exec(impl_->db,"DELETE FROM frozen_sync_members;DELETE FROM frozen_sync_batch;")!=ESP_OK||exec(impl_->db,"COMMIT;")!=ESP_OK)return rollback();
 return ESP_OK;
}
esp_err_t DurableStorage::recovery(size_t limit,const std::function<bool(const RecoveryItem&)>&visitor){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready()||!visitor||!limit)return ESP_ERR_INVALID_ARG;Statement s;const char*sql="SELECT o.operation_id,COALESCE(r.receipt_id,''),o.state,o.route_snapshot,COALESCE(r.canonical_payload,''),COALESCE(a.execution_unp,''),COALESCE(p.payment_id,''),COALESCE(p.amount_minor,0),COALESCE(p.state,''),COALESCE(p.terminal_reference,''),COALESCE(p.rrn,''),COALESCE(p.auth_code,''),o.updated_at,o.payload_digest FROM operations o LEFT JOIN receipts r ON r.operation_id=o.operation_id LEFT JOIN receipt_authority a ON a.receipt_id=r.receipt_id LEFT JOIN payments p ON p.payment_id=(SELECT p1.payment_id FROM payments p1 WHERE p1.operation_id=o.operation_id AND (p1.terminal_reference IS NOT NULL OR p1.state IN('CARD_UNKNOWN','CARD_APPROVED','EXECUTING')) ORDER BY CASE WHEN p1.terminal_reference IS NOT NULL THEN 0 ELSE 1 END,p1.updated_at DESC,p1.payment_id LIMIT 1) WHERE o.state NOT IN('COMMITTED','COMPENSATED','REJECTED') ORDER BY o.updated_at LIMIT ?";if(sqlite3_prepare_v2(impl_->db,sql,-1,&s.v,nullptr)!=SQLITE_OK)return ESP_FAIL;sqlite3_bind_int64(s.v,1,limit);int rc;while((rc=sqlite3_step(s.v))==SQLITE_ROW){RecoveryItem item{(const char*)sqlite3_column_text(s.v,0),(const char*)sqlite3_column_text(s.v,1),(const char*)sqlite3_column_text(s.v,2),(const char*)sqlite3_column_text(s.v,3),(const char*)sqlite3_column_text(s.v,4),(const char*)sqlite3_column_text(s.v,5),(const char*)sqlite3_column_text(s.v,6),sqlite3_column_int64(s.v,7),(const char*)sqlite3_column_text(s.v,8),(const char*)sqlite3_column_text(s.v,9),(const char*)sqlite3_column_text(s.v,10),(const char*)sqlite3_column_text(s.v,11),sqlite3_column_int64(s.v,12),(const char*)sqlite3_column_text(s.v,13)};if(!visitor(item))break;}return rc==SQLITE_DONE||rc==SQLITE_ROW?ESP_OK:ESP_FAIL;}
esp_err_t DurableStorage::prune_acknowledged(int64_t now,uint32_t days){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready()||now<=0||days<90)return ESP_ERR_INVALID_ARG;Statement s;if(sqlite3_prepare_v2(impl_->db,"DELETE FROM event_outbox WHERE acknowledged_at IS NOT NULL AND created_at<?",-1,&s.v,nullptr)!=SQLITE_OK)return ESP_FAIL;sqlite3_bind_int64(s.v,1,now-(int64_t)days*86400);return sqlite3_step(s.v)==SQLITE_DONE?ESP_OK:ESP_FAIL;}
int64_t DurableStorage::acknowledged_cursor()const{std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready())return-1;Statement s;if(sqlite3_prepare_v2(impl_->db,"SELECT acknowledged_sequence FROM sync_cursor WHERE singleton=1",-1,&s.v,nullptr)!=SQLITE_OK)return-1;return sqlite3_step(s.v)==SQLITE_ROW?sqlite3_column_int64(s.v,0):-1;}
std::string DurableStorage::acknowledged_event_hash()const{std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready())return{};Statement s;if(sqlite3_prepare_v2(impl_->db,"SELECT COALESCE(committed_event_hash,'') FROM sync_cursor WHERE singleton=1",-1,&s.v,nullptr)!=SQLITE_OK)return{};if(sqlite3_step(s.v)!=SQLITE_ROW)return{};auto*v=sqlite3_column_text(s.v,0);return v?(const char*)v:"";}
esp_err_t DurableStorage::configure_authority(const char*raw,int64_t first,int64_t last,int64_t generation){
 std::lock_guard<std::recursive_mutex> guard(impl_->mutex);
 if(!ready()||!raw||!authority_prefix_valid(raw)||first<1||last<first||last>9999999||generation<1)return ESP_ERR_INVALID_ARG;
 if(exec(impl_->db,"BEGIN IMMEDIATE;")!=ESP_OK)return ESP_FAIL;
 auto fail=[&](esp_err_t error){exec(impl_->db,"ROLLBACK;");return error;};
 if(exec(impl_->db,"INSERT OR IGNORE INTO authority_ranges(generation,prefix,range_start,range_end) SELECT generation,prefix,range_start,range_end FROM authority_state")!=ESP_OK)return fail(ESP_FAIL);
 Statement current;if(sqlite3_prepare_v2(impl_->db,"SELECT prefix,range_start,range_end,generation FROM authority_state WHERE singleton=1",-1,&current.v,nullptr)!=SQLITE_OK)return fail(ESP_FAIL);
 const int rc=sqlite3_step(current.v);bool same=false;
 if(rc==SQLITE_ROW){const std::string prefix=(const char*)sqlite3_column_text(current.v,0);same=prefix==raw&&first==sqlite3_column_int64(current.v,1)&&last==sqlite3_column_int64(current.v,2);const auto prior=sqlite3_column_int64(current.v,3);
  if(generation<prior||(generation==prior&&!same))return fail(ESP_ERR_INVALID_VERSION);
  if(generation==prior){const auto committed=exec(impl_->db,"COMMIT;");return committed==ESP_OK?ESP_OK:fail(committed);}
 }else if(rc!=SQLITE_DONE)return fail(ESP_FAIL);
 if(!same){Statement claims;if(sqlite3_prepare_v2(impl_->db,"SELECT 1 FROM sale_unp_claims WHERE serial=? AND sequence BETWEEN ? AND ? LIMIT 1",-1,&claims.v,nullptr)!=SQLITE_OK)return fail(ESP_FAIL);sqlite3_bind_text(claims.v,1,raw,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(claims.v,2,first);sqlite3_bind_int64(claims.v,3,last);if(sqlite3_step(claims.v)!=SQLITE_DONE)return fail(ESP_ERR_INVALID_VERSION);}
 if(!same){Statement overlap;if(sqlite3_prepare_v2(impl_->db,"SELECT 1 FROM authority_ranges WHERE (prefix=?1 OR (length(prefix)=14 AND substr(prefix,1,8)=?1)) AND range_start<=?2 AND range_end>=?3 LIMIT 1",-1,&overlap.v,nullptr)!=SQLITE_OK)return fail(ESP_FAIL);sqlite3_bind_text(overlap.v,1,raw,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(overlap.v,2,last);sqlite3_bind_int64(overlap.v,3,first);const int found=sqlite3_step(overlap.v);if(found==SQLITE_ROW)return fail(ESP_ERR_INVALID_VERSION);if(found!=SQLITE_DONE)return fail(ESP_FAIL);}
 Statement history,update;
 if(sqlite3_prepare_v2(impl_->db,"INSERT INTO authority_ranges(generation,prefix,range_start,range_end)VALUES(?,?,?,?)",-1,&history.v,nullptr)!=SQLITE_OK||sqlite3_prepare_v2(impl_->db,"INSERT INTO authority_state(singleton,prefix,range_start,range_end,next_unp,next_operation,generation)VALUES(1,?,?,?,?,1,?) ON CONFLICT(singleton)DO UPDATE SET prefix=excluded.prefix,range_start=excluded.range_start,range_end=excluded.range_end,next_unp=CASE WHEN ? THEN authority_state.next_unp ELSE excluded.range_start END,generation=excluded.generation",-1,&update.v,nullptr)!=SQLITE_OK)return fail(ESP_FAIL);
 sqlite3_bind_int64(history.v,1,generation);sqlite3_bind_text(history.v,2,raw,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(history.v,3,first);sqlite3_bind_int64(history.v,4,last);
 sqlite3_bind_text(update.v,1,raw,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(update.v,2,first);sqlite3_bind_int64(update.v,3,last);sqlite3_bind_int64(update.v,4,first);sqlite3_bind_int64(update.v,5,generation);sqlite3_bind_int(update.v,6,same?1:0);
 if(sqlite3_step(history.v)!=SQLITE_DONE||sqlite3_step(update.v)!=SQLITE_DONE||exec(impl_->db,"COMMIT;")!=ESP_OK){return fail(ESP_FAIL);}
 return ESP_OK;
}
esp_err_t DurableStorage::reserve_authority(AuthorityReservation&out,const char*operator_code){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);out={};if(!ready()||!operator_code||!authority_operator_valid(operator_code)||exec(impl_->db,"BEGIN IMMEDIATE;")!=ESP_OK)return ESP_FAIL;Statement q,u;if(sqlite3_prepare_v2(impl_->db,"SELECT prefix,next_unp,range_end,next_operation,range_start,generation FROM authority_state WHERE singleton=1",-1,&q.v,nullptr)!=SQLITE_OK||sqlite3_step(q.v)!=SQLITE_ROW){exec(impl_->db,"ROLLBACK;");return ESP_ERR_INVALID_STATE;}const int64_t unp=sqlite3_column_int64(q.v,1),last=sqlite3_column_int64(q.v,2),operation=sqlite3_column_int64(q.v,3);if(unp>last){exec(impl_->db,"ROLLBACK;");return ESP_ERR_NO_MEM;}std::string formatted;const char*prefix=(const char*)sqlite3_column_text(q.v,0);if(operation<1||operation>=std::numeric_limits<int64_t>::max()||!authority_unp(prefix,operator_code,unp,formatted)){exec(impl_->db,"ROLLBACK;");return ESP_ERR_INVALID_STATE;}out={operation,unp,formatted,prefix,sqlite3_column_int64(q.v,4),last,sqlite3_column_int64(q.v,5)};if(sqlite3_prepare_v2(impl_->db,"UPDATE authority_state SET next_unp=next_unp+1,next_operation=next_operation+1 WHERE singleton=1",-1,&u.v,nullptr)!=SQLITE_OK||sqlite3_step(u.v)!=SQLITE_DONE||exec(impl_->db,"COMMIT;")!=ESP_OK){exec(impl_->db,"ROLLBACK;");out={};return ESP_FAIL;}return ESP_OK;}
esp_err_t DurableStorage::load_local_sale(const char*id,std::string&payload,int64_t&version,std::string&state)const{std::lock_guard<std::recursive_mutex> guard(impl_->mutex);payload.clear();state.clear();version=0;if(!ready()||!id||!*id)return ESP_ERR_INVALID_ARG;Statement s;if(sqlite3_prepare_v2(impl_->db,"SELECT version,state,payload FROM local_sales WHERE surrogate_id=?",-1,&s.v,nullptr)!=SQLITE_OK)return ESP_FAIL;sqlite3_bind_text(s.v,1,id,-1,SQLITE_TRANSIENT);int rc=sqlite3_step(s.v);if(rc==SQLITE_DONE)return ESP_ERR_NOT_FOUND;if(rc!=SQLITE_ROW)return ESP_FAIL;version=sqlite3_column_int64(s.v,0);state=(const char*)sqlite3_column_text(s.v,1);payload=(const char*)sqlite3_column_text(s.v,2);return ESP_OK;}
esp_err_t DurableStorage::save_local_sale(const char*id,int64_t version,const char*state,const char*payload,int64_t now){std::lock_guard<std::recursive_mutex> guard(impl_->mutex);if(!ready()||!id||!*id||version<1||!state||!*state||!payload||now<=0)return ESP_ERR_INVALID_ARG;Statement s;const char*sql="INSERT INTO local_sales(surrogate_id,version,state,payload,created_at,updated_at)VALUES(?,?,?,?,?,?) ON CONFLICT(surrogate_id)DO UPDATE SET version=excluded.version,state=excluded.state,payload=excluded.payload,updated_at=excluded.updated_at WHERE local_sales.version<excluded.version";if(sqlite3_prepare_v2(impl_->db,sql,-1,&s.v,nullptr)!=SQLITE_OK)return ESP_FAIL;sqlite3_bind_text(s.v,1,id,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(s.v,2,version);sqlite3_bind_text(s.v,3,state,-1,SQLITE_TRANSIENT);sqlite3_bind_text(s.v,4,payload,-1,SQLITE_TRANSIENT);sqlite3_bind_int64(s.v,5,now);sqlite3_bind_int64(s.v,6,now);return sqlite3_step(s.v)==SQLITE_DONE&&sqlite3_changes(impl_->db)==1?ESP_OK:ESP_ERR_INVALID_STATE;}
}
