import { useLayoutEffect, useRef } from 'react';

/** Reject retained callbacks and admit only one command before React commits busy state. */
export function useCurrentAction() {
  const snapshot = {}, committed = useRef<object | null>(null), admitted = useRef<object | null>(null);
  useLayoutEffect(() => { committed.current = snapshot; return () => { if (committed.current === snapshot) committed.current = null; }; });
  return () => {
    if (committed.current !== snapshot || admitted.current === snapshot) return false;
    admitted.current = snapshot; return true;
  };
}
