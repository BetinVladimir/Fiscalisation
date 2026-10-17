type Device={id?:unknown;status?:unknown;model?:unknown};
/** Mirrors domain/admin.go identity constraints; backend remains authoritative. */
export function compositeSelectionValid(profile:string,adapter:string,fiscal:string,payment:string,devices:Device[]):boolean {
 const active=(id:string)=>{const matches=devices.filter(d=>d.id===id&&d.status==='ACTIVE');return id&&matches.length===1?matches[0]:undefined;};
 const adapterDevice=active(adapter);if(!adapterDevice||!active(fiscal)||(payment&&!active(payment)))return false;
 switch(profile){
  case 'DATECS_BLUECASH50_EMBEDDED':return adapter===fiscal&&payment===adapter&&typeof adapterDevice.model==='string'&&adapterDevice.model.toUpperCase()==='BLUECASH-50';
  case 'DATECS_DP150_BLUEPAD50':return Boolean(payment)&&adapter!==fiscal&&adapter!==payment;
  case 'DAISY_COMPACT_S01':return !payment&&adapter!==fiscal;
  default:return false;
 }
}
