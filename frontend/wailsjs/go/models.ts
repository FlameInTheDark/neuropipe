export namespace domain {
	
	export class APISettings {
	    enabled: boolean;
	    bindAddress: string;
	    port: number;
	    authMode: string;
	    tokenRef?: string;
	    allowedOrigins: string[];
	    adminEnabled: boolean;
	    exposureAcknowledged: boolean;
	
	    static createFrom(source: any = {}) {
	        return new APISettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.bindAddress = source["bindAddress"];
	        this.port = source["port"];
	        this.authMode = source["authMode"];
	        this.tokenRef = source["tokenRef"];
	        this.allowedOrigins = source["allowedOrigins"];
	        this.adminEnabled = source["adminEnabled"];
	        this.exposureAcknowledged = source["exposureAcknowledged"];
	    }
	}
	export class APIStatus {
	    running: boolean;
	    endpoint?: string;
	    tokenConfigured: boolean;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new APIStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.endpoint = source["endpoint"];
	        this.tokenConfigured = source["tokenConfigured"];
	        this.message = source["message"];
	    }
	}
	export class ChatToolCall {
	    id: string;
	    name: string;
	    arguments: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new ChatToolCall(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.arguments = source["arguments"];
	    }
	}
	export class ChatApproval {
	    id: string;
	    conversationId: string;
	    chatRunId: string;
	    toolCall: ChatToolCall;
	    status: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    resolvedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new ChatApproval(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.conversationId = source["conversationId"];
	        this.chatRunId = source["chatRunId"];
	        this.toolCall = this.convertValues(source["toolCall"], ChatToolCall);
	        this.status = source["status"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.resolvedAt = this.convertValues(source["resolvedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ChatConversation {
	    id: string;
	    mode: string;
	    title: string;
	    pipelineId?: string;
	    triggerBindingId?: string;
	    actionPolicy: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new ChatConversation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.mode = source["mode"];
	        this.title = source["title"];
	        this.pipelineId = source["pipelineId"];
	        this.triggerBindingId = source["triggerBindingId"];
	        this.actionPolicy = source["actionPolicy"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ChatMessage {
	    id: string;
	    conversationId: string;
	    chatRunId?: string;
	    role: string;
	    content: string;
	    toolCallId?: string;
	    toolName?: string;
	    toolCalls?: ChatToolCall[];
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new ChatMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.conversationId = source["conversationId"];
	        this.chatRunId = source["chatRunId"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.toolCallId = source["toolCallId"];
	        this.toolName = source["toolName"];
	        this.toolCalls = this.convertValues(source["toolCalls"], ChatToolCall);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ChatPipeline {
	    bindingId: string;
	    pipelineId: string;
	    pipelineName: string;
	    label: string;
	    icon: string;
	    color: string;
	    revision: number;
	
	    static createFrom(source: any = {}) {
	        return new ChatPipeline(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bindingId = source["bindingId"];
	        this.pipelineId = source["pipelineId"];
	        this.pipelineName = source["pipelineName"];
	        this.label = source["label"];
	        this.icon = source["icon"];
	        this.color = source["color"];
	        this.revision = source["revision"];
	    }
	}
	export class ChatRun {
	    id: string;
	    conversationId: string;
	    executionId?: string;
	    status: string;
	    statusText: string;
	    error?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new ChatRun(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.conversationId = source["conversationId"];
	        this.executionId = source["executionId"];
	        this.status = source["status"];
	        this.statusText = source["statusText"];
	        this.error = source["error"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ChatRunEvent {
	    id: string;
	    chatRunId: string;
	    kind: string;
	    summary: string;
	    detail?: string;
	    status?: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new ChatRunEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.chatRunId = source["chatRunId"];
	        this.kind = source["kind"];
	        this.summary = source["summary"];
	        this.detail = source["detail"];
	        this.status = source["status"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Option {
	    value: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new Option(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.label = source["label"];
	    }
	}
	export class ConfigField {
	    name: string;
	    label: string;
	    kind: string;
	    placeholder?: string;
	    options?: Option[];
	    required?: boolean;
	    secret?: boolean;
	    visibleWhen?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.label = source["label"];
	        this.kind = source["kind"];
	        this.placeholder = source["placeholder"];
	        this.options = this.convertValues(source["options"], Option);
	        this.required = source["required"];
	        this.secret = source["secret"];
	        this.visibleWhen = source["visibleWhen"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Viewport {
	    x: number;
	    y: number;
	    zoom: number;
	
	    static createFrom(source: any = {}) {
	        return new Viewport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.zoom = source["zoom"];
	    }
	}
	export class FlowEdge {
	    id: string;
	    source: string;
	    target: string;
	    sourceHandle?: string;
	    targetHandle?: string;
	    kind?: string;
	
	    static createFrom(source: any = {}) {
	        return new FlowEdge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.target = source["target"];
	        this.sourceHandle = source["sourceHandle"];
	        this.targetHandle = source["targetHandle"];
	        this.kind = source["kind"];
	    }
	}
	export class Position {
	    x: number;
	    y: number;
	
	    static createFrom(source: any = {}) {
	        return new Position(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	}
	export class FlowNode {
	    id: string;
	    type: string;
	    position: Position;
	    data: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new FlowNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.position = this.convertValues(source["position"], Position);
	        this.data = source["data"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FlowDefinition {
	    schemaVersion: number;
	    nodes: FlowNode[];
	    edges: FlowEdge[];
	    viewport: Viewport;
	
	    static createFrom(source: any = {}) {
	        return new FlowDefinition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.nodes = this.convertValues(source["nodes"], FlowNode);
	        this.edges = this.convertValues(source["edges"], FlowEdge);
	        this.viewport = this.convertValues(source["viewport"], Viewport);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FunctionPin {
	    id: string;
	    name: string;
	    dataType: string;
	    required?: boolean;
	    default?: any;
	
	    static createFrom(source: any = {}) {
	        return new FunctionPin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.dataType = source["dataType"];
	        this.required = source["required"];
	        this.default = source["default"];
	    }
	}
	export class CustomFunction {
	    id: string;
	    name: string;
	    description: string;
	    category: string;
	    icon: string;
	    iconColor: string;
	    iconBackground: string;
	    mode: string;
	    inputs: FunctionPin[];
	    outputs: FunctionPin[];
	    draftDefinition: FlowDefinition;
	    publishedRevision: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new CustomFunction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.category = source["category"];
	        this.icon = source["icon"];
	        this.iconColor = source["iconColor"];
	        this.iconBackground = source["iconBackground"];
	        this.mode = source["mode"];
	        this.inputs = this.convertValues(source["inputs"], FunctionPin);
	        this.outputs = this.convertValues(source["outputs"], FunctionPin);
	        this.draftDefinition = this.convertValues(source["draftDefinition"], FlowDefinition);
	        this.publishedRevision = source["publishedRevision"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DataField {
	    path: string;
	    label?: string;
	    dataType: string;
	    description?: string;
	    optional?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DataField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.label = source["label"];
	        this.dataType = source["dataType"];
	        this.description = source["description"];
	        this.optional = source["optional"];
	    }
	}
	export class DocumentationDocument {
	    id: string;
	    title: string;
	    summary?: string;
	    category: string[];
	    nodeTypes?: string[];
	    source: string;
	    pluginId?: string;
	    markdown: string;
	
	    static createFrom(source: any = {}) {
	        return new DocumentationDocument(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.category = source["category"];
	        this.nodeTypes = source["nodeTypes"];
	        this.source = source["source"];
	        this.pluginId = source["pluginId"];
	        this.markdown = source["markdown"];
	    }
	}
	export class DocumentationEntry {
	    id: string;
	    title: string;
	    summary?: string;
	    category: string[];
	    nodeTypes?: string[];
	    source: string;
	    pluginId?: string;
	
	    static createFrom(source: any = {}) {
	        return new DocumentationEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.category = source["category"];
	        this.nodeTypes = source["nodeTypes"];
	        this.source = source["source"];
	        this.pluginId = source["pluginId"];
	    }
	}
	export class DocumentationReference {
	    documentId: string;
	    anchor?: string;
	
	    static createFrom(source: any = {}) {
	        return new DocumentationReference(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.documentId = source["documentId"];
	        this.anchor = source["anchor"];
	    }
	}
	export class DocumentationSearchResult {
	    document: DocumentationEntry;
	    excerpt: string;
	
	    static createFrom(source: any = {}) {
	        return new DocumentationSearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.document = this.convertValues(source["document"], DocumentationEntry);
	        this.excerpt = source["excerpt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class NodeRun {
	    nodeId: string;
	    nodeType: string;
	    parentNodeId?: string;
	    functionId?: string;
	    functionRevision?: number;
	    status: string;
	    input?: any;
	    output?: any;
	    error?: string;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new NodeRun(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodeId = source["nodeId"];
	        this.nodeType = source["nodeType"];
	        this.parentNodeId = source["parentNodeId"];
	        this.functionId = source["functionId"];
	        this.functionRevision = source["functionRevision"];
	        this.status = source["status"];
	        this.input = source["input"];
	        this.output = source["output"];
	        this.error = source["error"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Execution {
	    id: string;
	    pipelineId: string;
	    triggerId?: string;
	    status: string;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    queuedAt?: any;
	    // Go type: time
	    runStartedAt?: any;
	    // Go type: time
	    finishedAt?: any;
	    error?: string;
	    nodeRuns?: NodeRun[];
	
	    static createFrom(source: any = {}) {
	        return new Execution(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.pipelineId = source["pipelineId"];
	        this.triggerId = source["triggerId"];
	        this.status = source["status"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.queuedAt = this.convertValues(source["queuedAt"], null);
	        this.runStartedAt = this.convertValues(source["runStartedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
	        this.error = source["error"];
	        this.nodeRuns = this.convertValues(source["nodeRuns"], NodeRun);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class FunctionSummary {
	    id: string;
	    name: string;
	    description: string;
	    category: string;
	    icon: string;
	    iconColor: string;
	    iconBackground: string;
	    mode: string;
	    publishedRevision: number;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new FunctionSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.category = source["category"];
	        this.icon = source["icon"];
	        this.iconColor = source["iconColor"];
	        this.iconBackground = source["iconBackground"];
	        this.mode = source["mode"];
	        this.publishedRevision = source["publishedRevision"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InstallProgress {
	    kind: string;
	    stage: string;
	    label: string;
	    downloadedBytes: number;
	    totalBytes: number;
	    bytesPerSecond: number;
	    percentage: number;
	
	    static createFrom(source: any = {}) {
	        return new InstallProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.stage = source["stage"];
	        this.label = source["label"];
	        this.downloadedBytes = source["downloadedBytes"];
	        this.totalBytes = source["totalBytes"];
	        this.bytesPerSecond = source["bytesPerSecond"];
	        this.percentage = source["percentage"];
	    }
	}
	export class InstalledLlamaRuntime {
	    version: string;
	    cpuInstalled: boolean;
	    cudaInstalled: boolean;
	    vulkanInstalled: boolean;
	    hipInstalled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InstalledLlamaRuntime(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.cpuInstalled = source["cpuInstalled"];
	        this.cudaInstalled = source["cudaInstalled"];
	        this.vulkanInstalled = source["vulkanInstalled"];
	        this.hipInstalled = source["hipInstalled"];
	    }
	}
	export class LlamaRuntimeCatalogStatus {
	    root: string;
	    selectedVersion?: string;
	    installed: InstalledLlamaRuntime[];
	
	    static createFrom(source: any = {}) {
	        return new LlamaRuntimeCatalogStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.selectedVersion = source["selectedVersion"];
	        this.installed = this.convertValues(source["installed"], InstalledLlamaRuntime);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LlamaRuntimeInstallRequest {
	    version: string;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new LlamaRuntimeInstallRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.mode = source["mode"];
	    }
	}
	export class RuntimeArtifact {
	    url?: string;
	    size: number;
	    sha256?: string;
	
	    static createFrom(source: any = {}) {
	        return new RuntimeArtifact(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.size = source["size"];
	        this.sha256 = source["sha256"];
	    }
	}
	export class LlamaRuntimeRelease {
	    version: string;
	    publishedAt?: string;
	    cpu: RuntimeArtifact;
	    cuda: RuntimeArtifact;
	    vulkan: RuntimeArtifact;
	    hip: RuntimeArtifact;
	
	    static createFrom(source: any = {}) {
	        return new LlamaRuntimeRelease(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.publishedAt = source["publishedAt"];
	        this.cpu = this.convertValues(source["cpu"], RuntimeArtifact);
	        this.cuda = this.convertValues(source["cuda"], RuntimeArtifact);
	        this.vulkan = this.convertValues(source["vulkan"], RuntimeArtifact);
	        this.hip = this.convertValues(source["hip"], RuntimeArtifact);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LlamaRuntimeSettings {
	    binaryPath: string;
	    modelPath: string;
	    runtimeVersion?: string;
	    mode: string;
	    contextSize: number;
	    autoStart: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LlamaRuntimeSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.binaryPath = source["binaryPath"];
	        this.modelPath = source["modelPath"];
	        this.runtimeVersion = source["runtimeVersion"];
	        this.mode = source["mode"];
	        this.contextSize = source["contextSize"];
	        this.autoStart = source["autoStart"];
	    }
	}
	export class LlamaRuntimeStatus {
	    running: boolean;
	    endpoint: string;
	    mode: string;
	    model: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LlamaRuntimeStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.endpoint = source["endpoint"];
	        this.mode = source["mode"];
	        this.model = source["model"];
	        this.message = source["message"];
	    }
	}
	export class LocalModel {
	    id: string;
	    name: string;
	    path: string;
	    size: number;
	    repository?: string;
	    author?: string;
	    avatarUrl?: string;
	    downloads: number;
	    likes: number;
	    lastModified?: string;
	    tags?: string[];
	    quantization?: string;
	    sha256?: string;
	    installedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalModel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.repository = source["repository"];
	        this.author = source["author"];
	        this.avatarUrl = source["avatarUrl"];
	        this.downloads = source["downloads"];
	        this.likes = source["likes"];
	        this.lastModified = source["lastModified"];
	        this.tags = source["tags"];
	        this.quantization = source["quantization"];
	        this.sha256 = source["sha256"];
	        this.installedAt = source["installedAt"];
	    }
	}
	export class MetricActivityEvent {
	    kind: string;
	    outcome: string;
	    durationMs: number;
	    // Go type: time
	    occurredAt: any;
	
	    static createFrom(source: any = {}) {
	        return new MetricActivityEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.outcome = source["outcome"];
	        this.durationMs = source["durationMs"];
	        this.occurredAt = this.convertValues(source["occurredAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MetricsBreakdown {
	    id: string;
	    label: string;
	    value: number;
	    secondary?: number;
	
	    static createFrom(source: any = {}) {
	        return new MetricsBreakdown(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.value = source["value"];
	        this.secondary = source["secondary"];
	    }
	}
	export class MetricsFilter {
	    // Go type: time
	    from: any;
	    // Go type: time
	    to: any;
	    pipelineIds?: string[];
	    providerIds?: string[];
	    models?: string[];
	    triggerKinds?: string[];
	    statuses?: string[];
	
	    static createFrom(source: any = {}) {
	        return new MetricsFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = this.convertValues(source["from"], null);
	        this.to = this.convertValues(source["to"], null);
	        this.pipelineIds = source["pipelineIds"];
	        this.providerIds = source["providerIds"];
	        this.models = source["models"];
	        this.triggerKinds = source["triggerKinds"];
	        this.statuses = source["statuses"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MetricsKPI {
	    value: number;
	    previousValue: number;
	    available: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MetricsKPI(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.previousValue = source["previousValue"];
	        this.available = source["available"];
	    }
	}
	export class MetricsResourcePoint {
	    // Go type: time
	    at: any;
	    process: string;
	    cpuPercent: number;
	    workingSet: number;
	
	    static createFrom(source: any = {}) {
	        return new MetricsResourcePoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = this.convertValues(source["at"], null);
	        this.process = source["process"];
	        this.cpuPercent = source["cpuPercent"];
	        this.workingSet = source["workingSet"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MetricsPipelineHealth {
	    pipelineId: string;
	    name: string;
	    // Go type: time
	    at: any;
	    completed: number;
	    failed: number;
	
	    static createFrom(source: any = {}) {
	        return new MetricsPipelineHealth(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pipelineId = source["pipelineId"];
	        this.name = source["name"];
	        this.at = this.convertValues(source["at"], null);
	        this.completed = source["completed"];
	        this.failed = source["failed"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MetricsPoint {
	    // Go type: time
	    at: any;
	    value: number;
	    value2?: number;
	    value3?: number;
	
	    static createFrom(source: any = {}) {
	        return new MetricsPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = this.convertValues(source["at"], null);
	        this.value = source["value"];
	        this.value2 = source["value2"];
	        this.value3 = source["value3"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MetricsRunPoint {
	    // Go type: time
	    at: any;
	    completed: number;
	    failed: number;
	    skipped: number;
	    cancelled: number;
	
	    static createFrom(source: any = {}) {
	        return new MetricsRunPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = this.convertValues(source["at"], null);
	        this.completed = source["completed"];
	        this.failed = source["failed"];
	        this.skipped = source["skipped"];
	        this.cancelled = source["cancelled"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MetricsOverview {
	    filter: MetricsFilter;
	    granularity: string;
	    runs: MetricsKPI;
	    successRate: MetricsKPI;
	    averageDurationMs: MetricsKPI;
	    p95DurationMs: MetricsKPI;
	    llmCalls: MetricsKPI;
	    promptTokens: MetricsKPI;
	    completionTokens: MetricsKPI;
	    estimatedCostUsd: MetricsKPI;
	    runSeries: MetricsRunPoint[];
	    durationSeries: MetricsPoint[];
	    llmSeries: MetricsPoint[];
	    queueSeries: MetricsPoint[];
	    pipelines: MetricsPipelineHealth[];
	    failures: MetricsBreakdown[];
	    slowNodes: MetricsBreakdown[];
	    models: MetricsBreakdown[];
	    triggers: MetricsBreakdown[];
	    activity: MetricsBreakdown[];
	    resources: MetricsResourcePoint[];
	    tokensUnavailable: number;
	    unpricedCalls: number;
	    localCalls: number;
	
	    static createFrom(source: any = {}) {
	        return new MetricsOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filter = this.convertValues(source["filter"], MetricsFilter);
	        this.granularity = source["granularity"];
	        this.runs = this.convertValues(source["runs"], MetricsKPI);
	        this.successRate = this.convertValues(source["successRate"], MetricsKPI);
	        this.averageDurationMs = this.convertValues(source["averageDurationMs"], MetricsKPI);
	        this.p95DurationMs = this.convertValues(source["p95DurationMs"], MetricsKPI);
	        this.llmCalls = this.convertValues(source["llmCalls"], MetricsKPI);
	        this.promptTokens = this.convertValues(source["promptTokens"], MetricsKPI);
	        this.completionTokens = this.convertValues(source["completionTokens"], MetricsKPI);
	        this.estimatedCostUsd = this.convertValues(source["estimatedCostUsd"], MetricsKPI);
	        this.runSeries = this.convertValues(source["runSeries"], MetricsRunPoint);
	        this.durationSeries = this.convertValues(source["durationSeries"], MetricsPoint);
	        this.llmSeries = this.convertValues(source["llmSeries"], MetricsPoint);
	        this.queueSeries = this.convertValues(source["queueSeries"], MetricsPoint);
	        this.pipelines = this.convertValues(source["pipelines"], MetricsPipelineHealth);
	        this.failures = this.convertValues(source["failures"], MetricsBreakdown);
	        this.slowNodes = this.convertValues(source["slowNodes"], MetricsBreakdown);
	        this.models = this.convertValues(source["models"], MetricsBreakdown);
	        this.triggers = this.convertValues(source["triggers"], MetricsBreakdown);
	        this.activity = this.convertValues(source["activity"], MetricsBreakdown);
	        this.resources = this.convertValues(source["resources"], MetricsResourcePoint);
	        this.tokensUnavailable = source["tokensUnavailable"];
	        this.unpricedCalls = source["unpricedCalls"];
	        this.localCalls = source["localCalls"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class ModelPriceRate {
	    providerId: string;
	    model: string;
	    inputUsdPerMillion: number;
	    outputUsdPerMillion: number;
	
	    static createFrom(source: any = {}) {
	        return new ModelPriceRate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.providerId = source["providerId"];
	        this.model = source["model"];
	        this.inputUsdPerMillion = source["inputUsdPerMillion"];
	        this.outputUsdPerMillion = source["outputUsdPerMillion"];
	    }
	}
	export class MetricsSettings {
	    detailRetentionDays: number;
	    rollupRetentionDays: number;
	    sampleIntervalSeconds: number;
	    priceRates: ModelPriceRate[];
	
	    static createFrom(source: any = {}) {
	        return new MetricsSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.detailRetentionDays = source["detailRetentionDays"];
	        this.rollupRetentionDays = source["rollupRetentionDays"];
	        this.sampleIntervalSeconds = source["sampleIntervalSeconds"];
	        this.priceRates = this.convertValues(source["priceRates"], ModelPriceRate);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ModelFile {
	    name: string;
	    size: number;
	    sha256?: string;
	    quantization?: string;
	    recommended?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModelFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.size = source["size"];
	        this.sha256 = source["sha256"];
	        this.quantization = source["quantization"];
	        this.recommended = source["recommended"];
	    }
	}
	export class ModelDetail {
	    id: string;
	    author?: string;
	    avatarUrl?: string;
	    downloads: number;
	    likes: number;
	    lastModified?: string;
	    tags?: string[];
	    readme?: string;
	    files: ModelFile[];
	
	    static createFrom(source: any = {}) {
	        return new ModelDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.author = source["author"];
	        this.avatarUrl = source["avatarUrl"];
	        this.downloads = source["downloads"];
	        this.likes = source["likes"];
	        this.lastModified = source["lastModified"];
	        this.tags = source["tags"];
	        this.readme = source["readme"];
	        this.files = this.convertValues(source["files"], ModelFile);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ModelInstallRequest {
	    repository: string;
	    file: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelInstallRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repository = source["repository"];
	        this.file = source["file"];
	    }
	}
	
	export class ModelSearchRequest {
	    query: string;
	    sort: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelSearchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.sort = source["sort"];
	    }
	}
	export class ModelSearchResult {
	    id: string;
	    author?: string;
	    avatarUrl?: string;
	    downloads: number;
	    likes: number;
	    lastModified?: string;
	    tags?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ModelSearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.author = source["author"];
	        this.avatarUrl = source["avatarUrl"];
	        this.downloads = source["downloads"];
	        this.likes = source["likes"];
	        this.lastModified = source["lastModified"];
	        this.tags = source["tags"];
	    }
	}
	export class NodePort {
	    id: string;
	    label: string;
	    kind: string;
	    direction: string;
	    dataType?: string;
	    fields?: DataField[];
	    color?: string;
	    required?: boolean;
	    default?: any;
	    maxConnections?: number;
	
	    static createFrom(source: any = {}) {
	        return new NodePort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.kind = source["kind"];
	        this.direction = source["direction"];
	        this.dataType = source["dataType"];
	        this.fields = this.convertValues(source["fields"], DataField);
	        this.color = source["color"];
	        this.required = source["required"];
	        this.default = source["default"];
	        this.maxConnections = source["maxConnections"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class NodeDefinition {
	    type: string;
	    category: string;
	    label: string;
	    description: string;
	    icon: string;
	    color: string;
	    mode: string;
	    inputs: NodePort[];
	    outputs: NodePort[];
	    fields: ConfigField[];
	    capabilities?: string[];
	    defaultConfig: Record<string, any>;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new NodeDefinition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.category = source["category"];
	        this.label = source["label"];
	        this.description = source["description"];
	        this.icon = source["icon"];
	        this.color = source["color"];
	        this.mode = source["mode"];
	        this.inputs = this.convertValues(source["inputs"], NodePort);
	        this.outputs = this.convertValues(source["outputs"], NodePort);
	        this.fields = this.convertValues(source["fields"], ConfigField);
	        this.capabilities = source["capabilities"];
	        this.defaultConfig = source["defaultConfig"];
	        this.source = source["source"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class PermissionGrant {
	    pipelineId: string;
	    revision: number;
	    capability: string;
	    scope: string;
	    // Go type: time
	    grantedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PermissionGrant(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pipelineId = source["pipelineId"];
	        this.revision = source["revision"];
	        this.capability = source["capability"];
	        this.scope = source["scope"];
	        this.grantedAt = this.convertValues(source["grantedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Pipeline {
	    id: string;
	    name: string;
	    description: string;
	    icon: string;
	    iconColor: string;
	    iconBackground: string;
	    status: string;
	    draftDefinition: FlowDefinition;
	    publishedRevision: number;
	    migrationIssue?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Pipeline(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.icon = source["icon"];
	        this.iconColor = source["iconColor"];
	        this.iconBackground = source["iconBackground"];
	        this.status = source["status"];
	        this.draftDefinition = this.convertValues(source["draftDefinition"], FlowDefinition);
	        this.publishedRevision = source["publishedRevision"];
	        this.migrationIssue = source["migrationIssue"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PipelineSummary {
	    id: string;
	    name: string;
	    description: string;
	    icon: string;
	    iconColor: string;
	    iconBackground: string;
	    status: string;
	    publishedRevision: number;
	    triggerCount: number;
	    migrationIssue?: string;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PipelineSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.icon = source["icon"];
	        this.iconColor = source["iconColor"];
	        this.iconBackground = source["iconBackground"];
	        this.status = source["status"];
	        this.publishedRevision = source["publishedRevision"];
	        this.triggerCount = source["triggerCount"];
	        this.migrationIssue = source["migrationIssue"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PluginStatus {
	    id: string;
	    name: string;
	    version: string;
	    path: string;
	    enabled: boolean;
	    healthy: boolean;
	    nodeCount: number;
	    description: string;
	    error?: string;
	    documentationError?: string;
	
	    static createFrom(source: any = {}) {
	        return new PluginStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.path = source["path"];
	        this.enabled = source["enabled"];
	        this.healthy = source["healthy"];
	        this.nodeCount = source["nodeCount"];
	        this.description = source["description"];
	        this.error = source["error"];
	        this.documentationError = source["documentationError"];
	    }
	}
	
	export class ProviderConfig {
	    id: string;
	    name: string;
	    kind: string;
	    baseUrl: string;
	    model: string;
	    apiKeyRef?: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProviderConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.baseUrl = source["baseUrl"];
	        this.model = source["model"];
	        this.apiKeyRef = source["apiKeyRef"];
	        this.enabled = source["enabled"];
	    }
	}
	export class Report {
	    id: string;
	    pipelineId: string;
	    pipelineName: string;
	    executionId: string;
	    nodeId: string;
	    title: string;
	    tags: string[];
	    markdown: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    executionStartedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.pipelineId = source["pipelineId"];
	        this.pipelineName = source["pipelineName"];
	        this.executionId = source["executionId"];
	        this.nodeId = source["nodeId"];
	        this.title = source["title"];
	        this.tags = source["tags"];
	        this.markdown = source["markdown"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.executionStartedAt = this.convertValues(source["executionStartedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Settings {
	    language: string;
	    defaultProviderId: string;
	    contentDirectory: string;
	    retentionDays: number;
	    webhookPort: number;
	    pluginDirectory: string;
	    providers: ProviderConfig[];
	    maxConcurrentRuns: number;
	    maxConcurrentLLMRuns: number;
	    llamaRuntime: LlamaRuntimeSettings;
	    api: APISettings;
	    metrics: MetricsSettings;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.defaultProviderId = source["defaultProviderId"];
	        this.contentDirectory = source["contentDirectory"];
	        this.retentionDays = source["retentionDays"];
	        this.webhookPort = source["webhookPort"];
	        this.pluginDirectory = source["pluginDirectory"];
	        this.providers = this.convertValues(source["providers"], ProviderConfig);
	        this.maxConcurrentRuns = source["maxConcurrentRuns"];
	        this.maxConcurrentLLMRuns = source["maxConcurrentLLMRuns"];
	        this.llamaRuntime = this.convertValues(source["llamaRuntime"], LlamaRuntimeSettings);
	        this.api = this.convertValues(source["api"], APISettings);
	        this.metrics = this.convertValues(source["metrics"], MetricsSettings);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TriggerBinding {
	    id: string;
	    pipelineId: string;
	    nodeId: string;
	    revision: number;
	    kind: string;
	    label: string;
	    icon: string;
	    color: string;
	    gridPosition: number;
	    hotkey?: string;
	    cron?: string;
	    timezone?: string;
	    enabled: boolean;
	    trusted: boolean;
	    // Go type: time
	    nextRunAt?: any;
	    // Go type: time
	    lastRunAt?: any;
	    lastRunStatus?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new TriggerBinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.pipelineId = source["pipelineId"];
	        this.nodeId = source["nodeId"];
	        this.revision = source["revision"];
	        this.kind = source["kind"];
	        this.label = source["label"];
	        this.icon = source["icon"];
	        this.color = source["color"];
	        this.gridPosition = source["gridPosition"];
	        this.hotkey = source["hotkey"];
	        this.cron = source["cron"];
	        this.timezone = source["timezone"];
	        this.enabled = source["enabled"];
	        this.trusted = source["trusted"];
	        this.nextRunAt = this.convertValues(source["nextRunAt"], null);
	        this.lastRunAt = this.convertValues(source["lastRunAt"], null);
	        this.lastRunStatus = source["lastRunStatus"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UpdateAvailability {
	    available: boolean;
	    version?: string;
	    url?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateAvailability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.version = source["version"];
	        this.url = source["url"];
	    }
	}

}

export namespace llm {
	
	export class ModelInfo {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}

}

export namespace security {
	
	export class SecretMetadata {
	    name: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new SecretMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

