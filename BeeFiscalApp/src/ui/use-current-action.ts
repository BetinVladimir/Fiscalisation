import { useEffect, useRef } from 'react';

/** Reject retained callbacks and admit only one command before React commits busy state. */
export function useCurrentAction() {
  const mounted = useRef(false), snapshot = {}, latest = useRef(snapshot), admitted = useRef<object | null>(null);
  latest.current = snapshot;
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  return () => {
    if (!mounted.current || latest.current !== snapshot || admitted.current === snapshot) return false;
    admitted.current = snapshot; return true;
  };
}
