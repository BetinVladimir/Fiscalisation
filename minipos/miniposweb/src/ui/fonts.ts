import regular from '../assets/fonts/Roboto_400Regular.ttf?url';
import medium from '../assets/fonts/Roboto_500Medium.ttf?url';
import bold from '../assets/fonts/Roboto_700Bold.ttf?url';
let pending:Promise<void>|null=null;
/** Rejections release the cache; retry uses fresh faces, not sticky failed CSS faces. */
export function loadRobotoFonts():Promise<void>{
 if(pending)return pending;
 const faces=[[400,regular],[500,medium],[700,bold]].map(([weight,url])=>new FontFace('Roboto',`url(${url})`,{weight:String(weight),style:'normal'}));
 const task=Promise.all(faces.map(face=>face.load())).then(loaded=>{for(const face of loaded){if(face.status!=='loaded')throw Error('Roboto registration failed');}for(const face of loaded)document.fonts.add(face);}).catch(error=>{if(pending===task)pending=null;throw error;});
 pending=task;return task;
}
