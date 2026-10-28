/** Reserve one visible title fragment; reduction is WEB-only and never below AA24. */
export function headerTargetSize(platform:string,rowWidth:number,icons:number){
 const count=Math.max(1,icons),gaps=Math.max(0,icons)*2,reservedTitle=16;
 if(platform!=='web'||rowWidth>=count*48+gaps+reservedTitle)return 48;
 return Math.max(24,Math.min(32,Math.floor((rowWidth-gaps-reservedTitle)/count)));
}
