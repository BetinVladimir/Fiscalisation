import type { Page } from '@playwright/test';

type AxeNode = {target:unknown[];any?:Array<{data?:{messageKey?:string}}>};
type AxeResult = {violations:Array<{id:string}>;incomplete:Array<{id:string;nodes:AxeNode[]}>};

/** Resolve only axe's RN-Web background-overlap uncertainty with a measured AA contrast check. */
export async function assertResolvedContrast(page:Page,scan:unknown){
  const result=scan as AxeResult;
  const unsupported=result.incomplete.filter(item=>item.id!=='color-contrast'||item.nodes.some(node=>!node.any?.every(check=>check.data?.messageKey==='bgOverlap'||check.data?.messageKey==='elmPartiallyObscured')));
  if(unsupported.length)throw Error(`Unresolved axe checks: ${unsupported.map(item=>item.id).join(', ')}`);
  const selectors=result.incomplete.flatMap(item=>item.nodes.map(node=>node.target.find(value=>typeof value==='string') as string|undefined).filter((value):value is string=>Boolean(value)));
  if(selectors.length!==result.incomplete.reduce((count,item)=>count+item.nodes.length,0))throw Error('WCAG AA contrast: unsupported target selector');
  const failures=await page.evaluate((targets:string[])=>{
    type Color={r:number;g:number;b:number;a:number};
    const parse=(value:string):Color|null=>{
      const match=value.match(/^rgba?\((\d+(?:\.\d+)?)[, ]+\s*(\d+(?:\.\d+)?)[, ]+\s*(\d+(?:\.\d+)?)(?:[, /]+\s*(\d+(?:\.\d+)?))?\)$/);
      return match?{r:+match[1],g:+match[2],b:+match[3],a:match[4]===undefined?1:+match[4]}:value==='transparent'?{r:0,g:0,b:0,a:0}:null;
    };
    const mix=(front:Color,back:Color):Color=>({r:front.r*front.a+back.r*(1-front.a),g:front.g*front.a+back.g*(1-front.a),b:front.b*front.a+back.b*(1-front.a),a:1});
    const luminance=(color:Color)=>{
      const channel=(v:number)=>{const s=v/255;return s<=0.04045?s/12.92:((s+0.055)/1.055)**2.4;};
      return 0.2126*channel(color.r)+0.7152*channel(color.g)+0.0722*channel(color.b);
    };
    const issues:string[]=[];
    for(const selector of targets){
      let node:HTMLElement|null=null;
      try{node=document.querySelector<HTMLElement>(selector);}catch{}
      if(!node){issues.push(`${selector}: target missing`);continue;}
      const layers:HTMLElement[]=[];for(let part:HTMLElement|null=node;part;part=part.parentElement)layers.unshift(part);
      let background:Color={r:255,g:255,b:255,a:1};
      for(const part of layers){const paint=parse(getComputedStyle(part).backgroundColor);if(paint)background=mix(paint,background);}
      const style=getComputedStyle(node),foreground=parse(style.color);
      if(!foreground){issues.push(`${selector}: foreground color unavailable`);continue;}
      const ink=mix(foreground,background),light=luminance(ink),dark=luminance(background);
      const ratio=(Math.max(light,dark)+0.05)/(Math.min(light,dark)+0.05);
      const size=parseFloat(style.fontSize),weight=Number(style.fontWeight)||(/bold/i.test(style.fontWeight)?700:400);
      const minimum=size>=24||size>=18.67&&weight>=700?3:4.5;
      if(!Number.isFinite(ratio)||ratio+0.01<minimum)issues.push(`${selector}: ${ratio.toFixed(2)} < ${minimum}`);
      const rect=node.getBoundingClientRect();
      if(rect.width&&rect.height&&rect.top>=0&&rect.left>=0&&rect.bottom<=innerHeight&&rect.right<=innerWidth){
        const top=document.elementFromPoint(rect.left+rect.width/2,rect.top+rect.height/2);
        if(top&&!node.contains(top)&&!top.contains(node))issues.push(`${selector}: obscured by ${top.tagName.toLowerCase()}`);
      }
    }
    return issues;
  },selectors);
  if(failures.length)throw Error(`WCAG AA contrast: ${failures.join('; ')}`);
}
