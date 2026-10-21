import {useLayoutEffect,useMemo,useRef,useState,type Dispatch,type SetStateAction} from 'react';

/** Results belong to a committed device context, including each A → B → A visit. */
export function useOwnedDeviceValue<T>(ownerKey:string,initial:T):[T,Dispatch<SetStateAction<T>>]{
  const empty=useRef(initial).current;
  const owner=useMemo(()=>({key:ownerKey}),[ownerKey]);
  const committed=useRef<object|null>(null);
  useLayoutEffect(()=>{committed.current=owner;return()=>{if(committed.current===owner)committed.current=null}},[owner]);
  const [stored,setStored]=useState<{owner:object;value:T}>(()=>({owner,value:empty}));
  const setValue:Dispatch<SetStateAction<T>>=next=>{
    if(committed.current!==owner)return;
    setStored(previous=>{
      if(committed.current!==owner)return previous;
      const value=previous.owner===owner?previous.value:empty;
      return {owner,value:typeof next==='function'?(next as (value:T)=>T)(value):next};
    });
  };
  return [stored.owner===owner?stored.value:empty,setValue];
}
