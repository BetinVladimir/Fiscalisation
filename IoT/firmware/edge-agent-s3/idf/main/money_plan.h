#pragma once
#include "intent_processor.h"
#include <limits>
#include <set>
namespace beefiscal::idf {
inline bool fixed_unsigned(const std::string&s,unsigned scale,int64_t&out){
 if(s.empty())return false;
 const auto dot=s.find('.');if(dot==std::string::npos||dot==0||s.size()-dot-1!=scale)return false;
 out=0;for(size_t i=0;i<s.size();i++){if(i==dot)continue;const char c=s[i];if(c<'0'||c>'9'||out>(std::numeric_limits<int64_t>::max()-(c-'0'))/10)return false;out=out*10+(c-'0');}return true;
}
inline bool valid_fiscal_password(EdgeProfile profile,const std::string&s){
 if(profile==EdgeProfile::DatecsDp150BluePad50?s.size()!=8:s.empty()||s.size()>6)return false;
 for(char c:s){if(c<'0'||c>'9')return false;}
 return true;
}
inline std::string execution_config_error(const CompositeBinding&b){
 const auto&c=b.fiscal.execution;
 if(!c.present||c.operator_number<1||c.operator_count<1||c.operator_count>30||c.operator_number>c.operator_count||c.operator_password_ref.empty()||c.operator_password_ref.size()>15||c.device_configuration_id.empty())return "FISCAL_CONFIG_REQUIRED";
 if(b.profile==EdgeProfile::DatecsDp150BluePad50){if(c.operator_count!=30||c.till_number<1||c.till_number>99999||(c.datecs_card_mode!=-1&&c.datecs_card_mode!=1&&c.datecs_card_mode!=2))return "FISCAL_CONFIG_INVALID";}
 else if(b.profile==EdgeProfile::DaisyCompactS01){if(c.till_number||(c.daisy_card_code&&std::string("NCDUBE").find(c.daisy_card_code)==std::string::npos))return "FISCAL_CONFIG_INVALID";}
 else return "FISCAL_CONFIG_INVALID";
 return {};
}
inline std::string receipt_plan_error(const CompositeBinding&b,const ReceiptPlan&p){
 const auto config=execution_config_error(b);if(!config.empty())return config;
 if(p.command_type!="SALE_FINALIZE"&&p.command_type!="SALE_REVERSE")return {};
 if(p.items.empty()||p.payments.empty())return "RECEIPT_EMPTY";
 int64_t due=0,paid=0;std::set<std::string>lines,payments;
 for(const auto&line:p.items){int64_t price,quantity,discount=0;
  if(line.id.empty()||!lines.insert(line.id).second||line.tax_group.size()!=1||line.tax_group[0]<'A'||line.tax_group[0]>'D'||!fixed_unsigned(line.unit_price,2,price)||!fixed_unsigned(line.quantity,3,quantity)||quantity<=0||price<=0||quantity>(std::numeric_limits<int64_t>::max()-500)/price||(!line.discount.empty()&&!fixed_unsigned(line.discount,2,discount)))return "RECEIPT_LINE_INVALID";
  const auto gross=(price*quantity+500)/1000;if(discount>gross||due>std::numeric_limits<int64_t>::max()-(gross-discount))return "RECEIPT_TOTAL_INVALID";due+=gross-discount;
 }
 for(const auto&payment:p.payments){int64_t amount;
  if(payment.id.empty()||!payments.insert(payment.id).second||(payment.type!="CASH"&&payment.type!="CARD")||!fixed_unsigned(payment.amount,2,amount)||amount<=0||amount!=payment.amount_minor||paid>std::numeric_limits<int64_t>::max()-amount)return "RECEIPT_PAYMENT_INVALID";
  if(payment.type=="CARD"&&((b.profile==EdgeProfile::DatecsDp150BluePad50&&b.fiscal.execution.datecs_card_mode<1)||(b.profile==EdgeProfile::DaisyCompactS01&&!b.fiscal.execution.daisy_card_code)))return "FISCAL_CARD_MAPPING_REQUIRED";
  paid+=amount;
 }
 return paid==due?std::string{}:"RECEIPT_PAYMENT_TOTAL_MISMATCH";
}
}
