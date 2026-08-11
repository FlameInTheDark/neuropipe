import {
  BaseEdge,
  EdgeLabelRenderer,
  Position,
  getBezierPath,
  useReactFlow,
  type EdgeProps,
} from "@xyflow/react";
import { GripVertical } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { FlowEdge } from "@/lib/types";

export const waypointMoveEvent = "neuropipe:move-wire-waypoint";
export const waypointRemoveEvent = "neuropipe:remove-wire-waypoint";

const waypointCoordinateLimit = 50_000;

interface Point {
  x: number;
  y: number;
}

/**
 * Shared editor edge. Wires without waypoints render exactly like the default
 * React Flow bezier edge; wires with waypoints route through each draggable
 * handle with smooth segments. Points are persisted graph layout metadata.
 */
export function BlueprintWaypointEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  markerEnd,
  style,
  data,
}: EdgeProps<FlowEdge>) {
  const { t } = useTranslation();
  const flow = useReactFlow();
  const waypoints: Point[] = Array.isArray(data?.waypoints)
    ? data.waypoints.filter(
        (point): point is Point =>
          typeof point === "object" &&
          point !== null &&
          Number.isFinite(point.x) &&
          Number.isFinite(point.y),
      )
    : [];

  const path =
    waypoints.length === 0
      ? bezierPath(
          sourceX,
          sourceY,
          sourcePosition,
          targetX,
          targetY,
          targetPosition,
        )
      : waypointPath(
          [{ x: sourceX, y: sourceY }, ...waypoints, { x: targetX, y: targetY }],
          sourcePosition,
          targetPosition,
        );

  const beginMove = (
    index: number,
    event: React.PointerEvent<HTMLButtonElement>,
  ) => {
    event.preventDefault();
    event.stopPropagation();
    event.currentTarget.setPointerCapture(event.pointerId);
    const move = (pointer: PointerEvent) => {
      const position = flow.screenToFlowPosition({
        x: pointer.clientX,
        y: pointer.clientY,
      });
      window.dispatchEvent(
        new CustomEvent(waypointMoveEvent, {
          detail: { edgeID: id, index, position: clampWaypoint(position) },
        }),
      );
    };
    const end = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", end);
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", end, { once: true });
  };

  const remove = (index: number, event: React.SyntheticEvent) => {
    event.preventDefault();
    event.stopPropagation();
    window.dispatchEvent(
      new CustomEvent(waypointRemoveEvent, { detail: { edgeID: id, index } }),
    );
  };

  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        markerEnd={markerEnd}
        style={style}
        interactionWidth={20}
      />
      {waypoints.map((point, index) => (
        <EdgeLabelRenderer key={`${id}-${index}`}>
          <button
            type="button"
            aria-label={t("canvas.moveWaypoint")}
            onPointerDown={(event) => beginMove(index, event)}
            onKeyDown={(event) => {
              if (event.key === "Delete" || event.key === "Backspace")
                remove(index, event);
            }}
            onDoubleClick={(event) => remove(index, event)}
            title={t("canvas.moveWaypoint")}
            className="nodrag nopan absolute z-10 flex size-5 cursor-grab touch-none items-center justify-center rounded-full border border-zinc-400 bg-zinc-950 text-zinc-400 shadow-md hover:border-zinc-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/60 active:cursor-grabbing"
            style={{
              transform: `translate(-50%, -50%) translate(${point.x}px, ${point.y}px)`,
              pointerEvents: "all",
            }}
          >
            <GripVertical className="size-3" />
          </button>
        </EdgeLabelRenderer>
      ))}
    </>
  );
}

function bezierPath(
  sourceX: number,
  sourceY: number,
  sourcePosition: Position | undefined,
  targetX: number,
  targetY: number,
  targetPosition: Position | undefined,
): string {
  const [path] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition: sourcePosition ?? Position.Right,
    targetX,
    targetY,
    targetPosition: targetPosition ?? Position.Left,
  });
  return path;
}

/**
 * Builds a cubic Hermite spline through source → waypoints → target. End
 * tangents follow the pins' natural directions so the wire exits and enters
 * exactly like the default bezier edge; each waypoint's tangent is the
 * bisector of its adjacent segments, which keeps the curve smooth through
 * every handle without loops or overshoot.
 */
function waypointPath(
  points: Point[],
  sourcePosition: Position | undefined,
  targetPosition: Position | undefined,
): string {
  if (points.length < 2) return "";
  const last = points.length - 1;
  const tangents: Point[] = new Array(points.length);
  tangents[0] = directionFor(sourcePosition ?? Position.Right);
  tangents[last] = reverse(directionFor(targetPosition ?? Position.Left));
  for (let index = 1; index < last; index++) {
    tangents[index] = bisector(
      points[index - 1],
      points[index],
      points[index + 1],
    );
  }
  const segments: string[] = [`M${points[0].x},${points[0].y}`];
  for (let index = 0; index < last; index++) {
    const start = points[index];
    const end = points[index + 1];
    const length = Math.hypot(end.x - start.x, end.y - start.y);
    if (length < 0.001) continue;
    // Capped at half the segment so short hops bend instead of overshooting.
    const control = Math.min(length / 2, Math.max(12, length / 3));
    const c1 = {
      x: start.x + tangents[index].x * control,
      y: start.y + tangents[index].y * control,
    };
    const c2 = {
      x: end.x - tangents[index + 1].x * control,
      y: end.y - tangents[index + 1].y * control,
    };
    segments.push(`C${c1.x},${c1.y} ${c2.x},${c2.y} ${end.x},${end.y}`);
  }
  return segments.join(" ");
}

/** Direction a bisector at a waypoint: average of normalized segment directions. */
function bisector(previous: Point, point: Point, next: Point): Point {
  const incoming = unit({ x: point.x - previous.x, y: point.y - previous.y });
  const outgoing = unit({ x: next.x - point.x, y: next.y - point.y });
  const sum = { x: incoming.x + outgoing.x, y: incoming.y + outgoing.y };
  if (Math.hypot(sum.x, sum.y) < 0.001) {
    // The wire doubles back on itself at this waypoint; turn perpendicular to
    // the incoming direction for a smooth U-turn instead of a loop.
    return { x: -incoming.y, y: incoming.x };
  }
  return unit(sum);
}

function directionFor(position: Position): Point {
  switch (position) {
    case Position.Left:
      return { x: -1, y: 0 };
    case Position.Right:
      return { x: 1, y: 0 };
    case Position.Top:
      return { x: 0, y: -1 };
    case Position.Bottom:
      return { x: 0, y: 1 };
  }
}

function reverse(point: Point): Point {
  return { x: -point.x, y: -point.y };
}

function unit(point: Point): Point {
  const length = Math.hypot(point.x, point.y);
  if (length < 0.001) return { x: 1, y: 0 };
  return { x: point.x / length, y: point.y / length };
}

function clampWaypoint(position: Point): Point {
  return {
    x: clampCoordinate(position.x),
    y: clampCoordinate(position.y),
  };
}

function clampCoordinate(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.min(
    waypointCoordinateLimit,
    Math.max(-waypointCoordinateLimit, value),
  );
}
