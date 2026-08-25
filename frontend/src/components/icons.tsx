import {
  Activity,
  AlignCenterHorizontal,
  AlignEndVertical,
  AlignLeft,
  AlignRight,
  AlignStartVertical,
  AlertTriangle,
  ArrowLeft,
  ArrowRightLeft,
  ArrowUpRight,
  Binary,
  Bold,
  BookOpen,
  Bot,
  Boxes,
  Braces,
  Cable,
  CalendarDays,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  CircleDot,
  CircleHelp,
  Clock,
  Code,
  Columns3,
  Command,
  Copy,
  CornerDownLeft,
  Crosshair,
  Database,
  Download,
  Expand,
  ExternalLink,
  Eye,
  EyeOff,
  FileArchive,
  FileCode2,
  FileDown,
  FileInput,
  FileOutput,
  FileQuestion,
  FileText,
  FileUp,
  FileX,
  Files,
  Filter,
  Fingerprint,
  FolderInput,
  FolderOpen,
  FolderX,
  Frame,
  Globe,
  GripVertical,
  Grid2x2,
  Hand,
  HardDrive,
  HelpCircle,
  History,
  Hourglass,
  Info,
  Italic,
  Keyboard,
  KeyRound,
  Layers,
  LayoutGrid,
  LayoutList,
  List,
  ListFilter,
  Loader2,
  Lock,
  LogIn,
  LogOut,
  Magnet,
  Maximize2,
  MessageSquare,
  MessagesSquare,
  Minus,
  MoreHorizontal,
  MousePointer2,
  Package,
  PackageOpen,
  PanelLeft,
  PanelRight,
  Pencil,
  PencilLine,
  Play,
  Plus,
  Radio,
  RefreshCw,
  Route,
  Save,
  Search,
  Send,
  Settings2,
  ShieldCheck,
  Sparkles,
  Split,
  Square,
  SquareFunction,
  StickyNote,
  Table2,
  TableProperties,
  Tags,
  TextCursorInput,
  Trash2,
  TrendingUp,
  UploadCloud,
  X,
  Zap,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

export const icons = {
  Activity, AlignCenterHorizontal, AlignEndVertical, AlignLeft, AlignRight, AlignStartVertical,
  AlertTriangle, ArrowLeft, ArrowRightLeft, ArrowUpRight,
  Binary, Bold, BookOpen, Bot, Boxes, Braces,
  Cable, CalendarDays, Check, ChevronDown, ChevronLeft, ChevronRight, CircleDot, CircleHelp, Clock, Code, Columns3, Command, Copy, CornerDownLeft, Crosshair,
  Database, Download,
  Expand, ExternalLink, Eye, EyeOff,
  FileArchive, FileCode2, FileDown, FileInput, FileOutput, FileQuestion, FileText, FileUp, FileX, Files, Filter, Fingerprint,
  FolderInput, FolderOpen, FolderX, Frame,
  Globe, GripVertical, Grid2x2,
  Hand, HardDrive, HelpCircle, History, Hourglass,
  Info, Italic,
  Keyboard, KeyRound,
  Layers, LayoutGrid, LayoutList, List, ListFilter, Loader2, Lock, LogIn, LogOut,
  Magnet, Maximize2, MessageSquare, MessagesSquare, Minus, MoreHorizontal, MousePointer2,
  Package, PackageOpen, PanelLeft, PanelRight, Pencil, PencilLine, Play, Plus,
  Radio, RefreshCw, Route,
  Save, Search, Send, Settings2, ShieldCheck, Sparkles,   Split, Square, SquareFunction, StickyNote,
  Table2, TableProperties, Tags, TextCursorInput, Trash2, TrendingUp,
  UploadCloud,
  X,
  Zap, ZoomIn, ZoomOut,
} satisfies Record<string, LucideIcon>;

export type IconName = keyof typeof icons;

/** Converts backend kebab-case icon names ("arrow-right-left") to PascalCase ("ArrowRightLeft"). */
function toPascalCase(kebab: string): string {
  return kebab
    .split("-")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join("");
}

/** Known backend names that don't map to a Lucide export via simple PascalCase. */
const ICON_ALIASES: Record<string, string> = {
  "folder-copy": "Files",
  "folder-question": "HelpCircle",
};

export function Icon({
  name,
  className = "h-4 w-4",
  strokeWidth = 1.75,
}: {
  name: string;
  className?: string;
  strokeWidth?: number;
}) {
  // Try exact PascalCase first, then normalize from kebab-case, then aliases
  const Cmp =
    (icons as Record<string, LucideIcon>)[name] ??
    (icons as Record<string, LucideIcon>)[toPascalCase(name)] ??
    (icons as Record<string, LucideIcon>)[ICON_ALIASES[name]] ??
    Boxes;
  return <Cmp className={className} strokeWidth={strokeWidth} />;
}

