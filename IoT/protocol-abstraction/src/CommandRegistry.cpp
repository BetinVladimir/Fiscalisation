#include "FiscalDriver.hpp"

namespace bee { namespace {
bool oneOf(uint16_t value,const uint16_t* values,size_t count){for(size_t i=0;i<count;i++)if(values[i]==value)return true;return false;}

template<size_t N> bool oneOf(uint16_t value,const uint16_t(&values)[N]){return oneOf(value,values,N);}

const char* canonical(uint16_t code){
 switch(code){
  case 33:case 35:case 47:case 63:return "DisplayControl"; case 43:return "CreateReversal"; case 44:return "PaperFeed"; case 45:return "Probe"; case 46:return "PaperCut"; case 48:return "OpenFiscalReceipt";
  case 49:case 52:case 58:case 138:return "AddItem"; case 50:case 97:return "GetTaxRates";
  case 51:return "GetSubtotal/AddAdjustment"; case 53:return "AddPayment"; case 56:return "CloseReceipt"; case 57:return "SetInvoiceData";
  case 60:case 130:return "CancelReceipt"; case 62:return "GetClock"; case 64:case 113:return "LookupLastReceipt";
  case 69:case 108:return "XReport/ZReport"; case 70:return "CashIn/CashOut"; case 71:return "GetNapTransmissionStatus/GetDiagnosticInfo"; case 74:return "GetStatus/Probe";
  case 76:case 103:case 119:case 125:return "GetReceiptInfo"; case 73:case 79:case 94:case 95:return "FiscalMemoryReport"; case 80:return "Sound"; case 84:return "PrintBarcode";
  case 90:case 174:return "GetDiagnosticInfo"; case 99:return "GetDeviceIdentity"; case 105:return "OperatorReport";
  case 92:return "PrintSeparator"; case 106:return "OpenDrawer"; case 110:return "GetDailyInfo"; case 111:return "PluReport"; case 112:return "GetOperator";
  case 116:return "FiscalMemoryExport"; case 117:return "GetNapTransmissionStatus"; case 135:return "GetDiagnosticInfo"; case 140:return "ClientDirectory";
  case 118:case 123:return "GetFirmwareInfo/GetCapabilities"; case 128:return "GetCapabilities";
  case 153:return "ExportDeviceReport"; case 165:return "DepartmentReport"; case 166:return "SystemParametersReport";
  case 201:return "GetCurrencyTransition"; default:return "VendorCommand";
 }
}

RetryClass daisyRetry(uint16_t code){
 const uint16_t read[]={50,62,64,65,66,68,74,76,90,97,99,103,110,112,113,114,116,117,118,119,128,146,153,173,174,194,201};
 const uint16_t never[]={42,54,55,71,73,79,84,85,94,95,96,101,102,104,105,109,111,115,132,149,154,155,156,157,165,166,176,195};
 const uint16_t pre[]={33,35,38,44,45,46,47,63,100,106,133};
 if(oneOf(code,read))return RetryClass::Read;if(oneOf(code,never))return RetryClass::Never;if(oneOf(code,pre))return RetryClass::PreAccept;return RetryClass::LookupThenDecide;
}
Disposition daisyDisposition(uint16_t code){
 const uint16_t optional[]={33,35,44,45,46,47,57,63,71,84,85,106,133,152,173,176};
 const uint16_t privileged[]={38,39,42,43,54,61,96,101,102,104,107,109,131,150,151,195};
 const uint16_t excluded[]={55,100,115,132,134,149,154,155,156,157,194};
 if(oneOf(code,optional))return Disposition::Optional;if(oneOf(code,privileged))return Disposition::Privileged;if(oneOf(code,excluded))return Disposition::Excluded;return Disposition::Supported;
}
RetryClass datecsRetry(uint16_t code){
 const uint16_t read[]={45,50,62,64,65,68,71,74,76,86,87,88,90,99,100,103,110,112,115,116,123,124,125,126,135};
 const uint16_t never[]={42,54,61,66,72,83,84,89,91,92,94,95,96,98,101,105,107,109,111,122,127,202,203,253,255};
 const uint16_t pre[]={33,35,38,44,46,47,63,80,106};
 if(oneOf(code,read))return RetryClass::Read;if(oneOf(code,never))return RetryClass::Never;if(oneOf(code,pre))return RetryClass::PreAccept;return RetryClass::LookupThenDecide;
}
Disposition datecsDisposition(uint16_t code){
 const uint16_t optional[]={33,35,44,46,47,57,63,80,84,92,106,140};
 const uint16_t privileged[]={38,39,42,54,61,66,83,89,101,107,109,122};
 const uint16_t excluded[]={55,72,91,96,98,127,202,203,253,255};
 if(oneOf(code,optional))return Disposition::Optional;if(oneOf(code,privileged))return Disposition::Privileged;if(oneOf(code,excluded))return Disposition::Excluded;return Disposition::Supported;
}
}

bool daisyCommandSpec(uint16_t code,CommandSpec& out){
 if(!isDaisyDocumented(code))return false;
 out=CommandSpec{code,"Daisy documented command",canonical(code),daisyDisposition(code),daisyRetry(code)};
 return true;
}
bool datecsCommandSpec(uint16_t code,CommandSpec& out){
 if(!isDatecsDocumented(code))return false;
 out=CommandSpec{code,"Datecs documented command",canonical(code),datecsDisposition(code),datecsRetry(code)};
 return true;
}
}
