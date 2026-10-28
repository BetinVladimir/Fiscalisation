import {useEffect,useLayoutEffect,useRef} from 'react';
import {Platform} from 'react-native';

/** Adapt the real Paper modal DOM; never substitute its controls or surface. */
export function useShiftDialogAccessibility(visible:boolean,testID:string,busy:boolean,onCancel:()=>void,returnFocus?:()=>HTMLElement|null){
 const current=useRef({busy,onCancel,returnFocus});
 // Keyboard handlers belong to the committed dialog, including during Suspense.
 useLayoutEffect(()=>{current.current={busy,onCancel,returnFocus};},[busy,onCancel,returnFocus]);
 // Updating busy must not release modal isolation or move focus.
 useEffect(()=>{if(visible&&Platform.OS==='web'&&typeof document!=='undefined')document.querySelector<HTMLElement>(`[data-testid="${testID}-surface"]`)?.setAttribute('aria-busy',String(busy));},[visible,testID,busy]);
 useEffect(()=>{
  if(!visible||Platform.OS!=='web'||typeof document==='undefined')return;
  const previous=document.activeElement instanceof HTMLElement?document.activeElement:null;
  const originals=new Map<HTMLElement,{inert:boolean;hidden:string|null;visibility:string;priority:string}>();let focused=false;
  const surface=()=>document.querySelector<HTMLElement>(`[data-testid="${testID}-surface"]`);
  const controls=()=>Array.from(surface()?.querySelectorAll<HTMLElement>('button,a[href],input,select,textarea,[tabindex]')??[]).filter(node=>node.tabIndex>=0&&!node.hasAttribute('disabled')&&node.getAttribute('aria-disabled')!=='true'&&node.getClientRects().length>0);
  const prepare=()=>{
   const body=surface(),root=document.querySelector<HTMLElement>(`[data-testid="${testID}"]`),title=document.querySelector<HTMLElement>(`[data-testid="${testID}-title"]`),description=document.querySelector<HTMLElement>(`[data-testid="${testID}-description"]`);
   if(!body||!root||!title)return;
   title.id=`${testID}-title`;body.setAttribute('role','dialog');body.setAttribute('aria-modal','true');body.setAttribute('aria-busy',String(current.current.busy));body.setAttribute('aria-labelledby',title.id);body.tabIndex=-1;
   if(description){description.id=`${testID}-description`;body.setAttribute('aria-describedby',description.id);}
   for(let branch:HTMLElement|null=root;branch?.parentElement;branch=branch.parentElement){
    for(const sibling of Array.from(branch.parentElement.children)){
     if(sibling===branch||!(sibling instanceof HTMLElement)||/^(SCRIPT|STYLE|LINK)$/.test(sibling.tagName))continue;
     if(!originals.has(sibling))originals.set(sibling,{inert:sibling.inert,hidden:sibling.getAttribute('aria-hidden'),visibility:sibling.style.getPropertyValue('visibility'),priority:sibling.style.getPropertyPriority('visibility')});
     sibling.style.setProperty('visibility','hidden');
     if(!sibling.inert)sibling.inert=true;
     if(sibling.getAttribute('aria-hidden')!=='true')sibling.setAttribute('aria-hidden','true');
    }
    if(branch.parentElement===document.body)break;
   }
   if(!focused){(controls()[0]??body).focus({preventScroll:true});focused=true;}
  };
  const key=(event:KeyboardEvent)=>{
   if(event.key==='Escape'){event.preventDefault();event.stopPropagation();if(!current.current.busy)current.current.onCancel();return;}
   if(event.key!=='Tab')return;
   const list=controls(),body=surface();if(!body)return;
   if(!list.length){event.preventDefault();body.focus();return;}
   const index=list.findIndex(node=>node===document.activeElement);
   if(index<0||(event.shiftKey&&index===0)||(!event.shiftKey&&index===list.length-1)){event.preventDefault();list[event.shiftKey?list.length-1:0].focus();}
  };
  const focus=(event:FocusEvent)=>{const body=surface();if(body&&event.target instanceof Node&&!body.contains(event.target))(controls()[0]??body).focus({preventScroll:true});};
  const observer=new MutationObserver(prepare);observer.observe(document.body,{childList:true,subtree:true});prepare();document.addEventListener('keydown',key,true);document.addEventListener('focusin',focus,true);
  return()=>{observer.disconnect();document.removeEventListener('keydown',key,true);document.removeEventListener('focusin',focus,true);for(const [node,original] of originals){node.inert=original.inert;if(original.visibility)node.style.setProperty('visibility',original.visibility,original.priority);else node.style.removeProperty('visibility');if(original.hidden===null)node.removeAttribute('aria-hidden');else node.setAttribute('aria-hidden',original.hidden);}if(current.current.returnFocus){const target=current.current.returnFocus();if(target?.isConnected&&!target.hasAttribute('disabled')&&target.getAttribute('aria-disabled')!=='true'&&!target.closest('[inert],[hidden],[aria-hidden="true"]')&&target.getClientRects().length>0)target.focus({preventScroll:true});}else if(previous?.isConnected)previous.focus({preventScroll:true});};
 },[visible,testID]);
}
