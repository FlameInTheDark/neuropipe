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
import { Tooltip } from "@/components/ui/tooltip";
import type { FlowEdge } from "@/lib/types";

export const waypointMoveEvent = "neuropipe:move-wire-waypoint";
export const waypointRemoveEvent = "neuropipe:remove-wire-waypoint";

const waypointCoordinateLimit = 50_000;

// Matches React Flow's default bezier curvature so segments leaving or
// entering a waypoint bow exactly like a direct pin-to-pin connection.
const wireCurvature = 0.25;

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
          <div
            className="nodrag nopan absolute z-10"
            style={{
              transform: `translate(-50%, -50%) translate(${point.x}px, ${point.y}px)`,
              pointerEvents: "all",
            }}
          >
            <Tooltip content={t("canvas.moveWaypoint")} side="top" wrap={false}>
              <button
                type="button"
                aria-label={t("canvas.moveWaypoint")}
                onPointerDown={(event) => beginMove(index, event)}
                onKeyDown={(event) => {
                  if (event.key === "Delete" || event.key === "Backspace")
                    remove(index, event);
                }}
                onDoubleClick={(event) => remove(index, event)}
                className="flex size-5 cursor-grab touch-none items-center justify-center rounded-full border border-zinc-400 bg-zinc-950 text-zinc-400 shadow-md hover:border-zinc-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/60 active:cursor-grabbing"
              >
                <GripVertical className="size-3" />
              </button>
            </Tooltip>
          </div>
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
 * Routes source → waypoints → target as one curve per segment. Only the first
 * and last segments bend: the first leaves the source pin along its handle
 * direction and the last enters the target pin along its handle direction,
 * exactly like a direct pin-to-pin bezier. Waypoint-to-waypoint and
 * waypoint-to-target runs that are already straight render as straight lines —
 * each segment simply travels along its own direction, and adjacent segments
 * share that direction at the join so bends stay smooth.
 */
function waypointPath(
  points: Point[],
  sourcePosition: Position | undefined,
  targetPosition: Position | undefined,
): string {
  if (points.length < 2) return "";
  const last = points.length - 1;
  const segments: string[] = [`M${points[0].x},${points[0].y}`];
  for (let index = 0; index < last; index++) {
    const start = points[index];
    const end = points[index + 1];
    const length = Math.hypot(end.x - start.x, end.y - start.y);
    if (length < 0.001) continue;
    const travel = unit({ x: end.x - start.x, y: end.y - start.y });
    const startDir = index === 0
      ? directionFor(sourcePosition ?? Position.Right)
      : travel;
    const endDir = index === last
      ? reverse(directionFor(targetPosition ?? Position.Left))
      : travel;
    // The same curvature factor React Flow's default bezier uses for direct
    // pin-to-pin wires, so a rerouted wire bends no more than a direct one.
    const control = length * wireCurvature;
    const c1 = {
      x: start.x + startDir.x * control,
      y: start.y + startDir.y * control,
    };
    const c2 = {
      x: end.x - endDir.x * control,
      y: end.y - endDir.y * control,
    };
    segments.push(`C${c1.x},${c1.y} ${c2.x},${c2.y} ${end.x},${end.y}`);
  }
  return segments.join(" ");
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
