// Synthetic presentation evidence only: these values do not prove fiscal/BLE workflows.
import{parseApiJson}from'../src/parseApiJson';
const common={future_field:'FUTURE_VALUE_PRESERVED',sequence:'9223372036854775807',empty:{},none:null,flag:false};
export const deviceDataSections=[
 {id:'smart-device-activation-preview',title:'activationRequest',value:{...common,activation_request_id:'synthetic-request',device_instance_id:'synthetic-device',device_key_thumbprint:'a'.repeat(64),vendor:'Vendor',model:'Model',serial:'00007',fmin:'FM00007',capability_digest:'b'.repeat(64),state:'PENDING',requested_roles:['FISCAL_DEVICE'],expires_at:'2099-01-01T00:00:00Z',created_at:'2026-10-18T00:00:00Z',updated_at:'2026-10-18T00:00:00Z'}},
 {id:'transport-metrics',title:'transport',value:{...common,state:'READY',hops:{cloud:{state:'READY'},edge:{state:'READY'}},recommended_transport:'REST'}},
 {id:'device-realtime-health',title:'health',value:{...common,...parseApiJson('{"sequence":9223372036854775807,"binding_generation":-9223372036854775808}'),id:'synthetic-health',version:1,adapter_device_id:'adapter',register_id:'register',boot_id:'boot',adapter_state:'READY',endpoints:{fiscal:'SYNTHETIC_ENDPOINT'},observed_at:'2026-10-18T00:00:00Z',received_at:'2026-10-18T00:00:01Z',effective_state:'READY',age_seconds:1}},
 {id:'device-activity-timeline',title:'activity',value:Array.from({length:31},(_,i)=>({...common,id:'transition-'+i,state:'STATE_'+i,observed_at:'2026-10-18T00:00:00Z'}))},
 {id:'fiscal-device-metrics',title:'fiscal',value:{...common,ready:true,components:{fiscal_device:'READY'},observed_at:'2026-10-18T00:00:00Z'}},
 {id:'diagnostics-result',title:'diagnostics',value:{...common,capabilities:{device_id:'device',driver_version:'1',protocol_version:3,capabilities:{printer_test:true},evidence_state:'READY'},details:{device_id:'device',observed_at:'2026-10-18T00:00:00Z',metrics:{safe:0},redactions_applied:true}}},
 {id:'provisioning-result',title:'provisioning',value:{...common,state:'INSTALLED',binding_id:'binding',binding_sha256:'c'.repeat(64),generation:7}},
 {id:'ble-session-result',title:'ble',value:{...common,ble_session_id:'session',tenant_id:'tenant',edge_id:'edge',device_id:'device',location_id:'location',register_id:'register',service_uuid:'00000000-0000-4000-8000-000000000001',security_mode:'SIGNED',signed_session_ticket:'SYNTHETIC_TICKET_'+('X'.repeat(128)),expires_at:'2099-01-01T00:00:00Z'}},
 {id:'bluecash-activation-result',title:'activation',value:{...common,state:'ACTIVE',claimed_tenant_id:'tenant',location_id:'location',register_id:'register',roles:['FISCAL_DEVICE'],binding_version:1}},
]as const;
