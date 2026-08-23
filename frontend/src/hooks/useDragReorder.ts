import { useCallback, useRef, useState } from "react";

/** Moves the item with `sourceId` to the index currently held by `targetId`. */
export function reorderById<T extends { id: string }>(list: T[], sourceId: string, targetId: string): T[] {
  if (sourceId === targetId) return list;
  const from = list.findIndex((i) => i.id === sourceId);
  const to = list.findIndex((i) => i.id === targetId);
  if (from < 0 || to < 0) return list;
  const next = [...list];
  const [item] = next.splice(from, 1);
  next.splice(to, 0, item);
  return next;
}

/** Moves the item with `sourceId` to the end of the list. */
export function moveToEndById<T extends { id: string }>(list: T[], sourceId: string): T[] {
  const from = list.findIndex((i) => i.id === sourceId);
  if (from < 0) return list;
  const next = [...list];
  const [item] = next.splice(from, 1);
  next.push(item);
  return next;
}

/**
 * HTML5 drag-and-drop reordering.
 * Returns prop-getters so call sites stay declarative instead of
 * repeating six drag handlers per item.
 */
export function useDragReorder(mime: string, onReorder: (sourceId: string, targetId: string) => void) {
  const [dragging, setDragging] = useState<string | null>(null);
  const [over, setOver] = useState<string | null>(null);
  /** true while a drag is in flight, so click handlers can ignore the drop */
  const suppressClick = useRef(false);

  const end = useCallback(() => {
    setDragging(null);
    setOver(null);
    window.setTimeout(() => {
      suppressClick.current = false;
    }, 0);
  }, []);

  const readSource = useCallback(
    (e: React.DragEvent) => e.dataTransfer.getData(mime) || dragging,
    [mime, dragging],
  );

  /** spread onto each draggable item */
  const itemProps = useCallback(
    (id: string) => ({
      draggable: true,
      onDragStart: (e: React.DragEvent) => {
        suppressClick.current = true;
        setDragging(id);
        e.dataTransfer.effectAllowed = "move";
        e.dataTransfer.setData(mime, id);
      },
      onDragEnter: (e: React.DragEvent) => {
        e.preventDefault();
        if (dragging && dragging !== id) setOver(id);
      },
      onDragOver: (e: React.DragEvent) => {
        e.preventDefault();
        e.dataTransfer.dropEffect = "move";
      },
      onDrop: (e: React.DragEvent) => {
        e.preventDefault();
        const source = readSource(e);
        if (source) onReorder(source, id);
        end();
      },
      onDragEnd: end,
    }),
    [mime, dragging, onReorder, readSource, end],
  );

  return {
    dragging,
    over,
    itemProps,
    readSource,
    end,
    /** call at the top of a click handler to ignore clicks that ended a drag */
    consumeClickAfterDrag: () => {
      if (!suppressClick.current) return false;
      suppressClick.current = false;
      return true;
    },
  };
}
