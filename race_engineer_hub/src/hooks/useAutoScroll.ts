import { useCallback, useEffect, useRef } from "react";

/**
 * Auto-scrolls a container to the bottom only when the user is already
 * near the bottom. If the user has scrolled up, new content won't yank
 * them back down.
 *
 * Uses a callback ref so the scroll listener is correctly attached even
 * for conditionally rendered elements (e.g. expanded overlays).
 *
 * @param dep - reactive value that triggers the scroll check (e.g. items.length)
 * @param threshold - pixel tolerance for "near bottom" (default 50)
 */
export function useAutoScroll<T extends HTMLElement>(dep: unknown, threshold = 50) {
  const elRef = useRef<T | null>(null);
  const isNearBottom = useRef(true);
  const cleanupRef = useRef<(() => void) | null>(null);

  const callbackRef = useCallback(
    (node: T | null) => {
      // Detach previous listener
      cleanupRef.current?.();
      cleanupRef.current = null;
      elRef.current = node;

      if (node) {
        isNearBottom.current = node.scrollHeight - node.scrollTop - node.clientHeight <= threshold;
        const onScroll = () => {
          isNearBottom.current = node.scrollHeight - node.scrollTop - node.clientHeight <= threshold;
        };
        node.addEventListener("scroll", onScroll, { passive: true });
        cleanupRef.current = () => node.removeEventListener("scroll", onScroll);
      }
    },
    [threshold],
  );

  useEffect(() => {
    if (isNearBottom.current && elRef.current) {
      elRef.current.scrollTop = elRef.current.scrollHeight;
    }
  }, [dep]);

  return callbackRef;
}
