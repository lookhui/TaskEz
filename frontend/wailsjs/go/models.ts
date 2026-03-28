export namespace main {
	
	export class ScheduledTaskInfo {
	    name: string;
	    path: string;
	    state: string;
	    author: string;
	    description: string;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new ScheduledTaskInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.state = source["state"];
	        this.author = source["author"];
	        this.description = source["description"];
	        this.command = source["command"];
	    }
	}
	export class DriverInfo {
	    name: string;
	    displayName: string;
	    state: string;
	    startMode: string;
	    path: string;
	    serviceType: string;
	
	    static createFrom(source: any = {}) {
	        return new DriverInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.state = source["state"];
	        this.startMode = source["startMode"];
	        this.path = source["path"];
	        this.serviceType = source["serviceType"];
	    }
	}
	export class InventorySnapshot {
	    generatedAt: string;
	    drivers: DriverInfo[];
	    tasks: ScheduledTaskInfo[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new InventorySnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.generatedAt = source["generatedAt"];
	        this.drivers = this.convertValues(source["drivers"], DriverInfo);
	        this.tasks = this.convertValues(source["tasks"], ScheduledTaskInfo);
	        this.warnings = source["warnings"];
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
	export class WatchEvent {
	    time: string;
	    source: string;
	    action: string;
	    target: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new WatchEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.source = source["source"];
	        this.action = source["action"];
	        this.target = source["target"];
	        this.detail = source["detail"];
	    }
	}
	export class MonitorState {
	    startedAt: string;
	    watchedPaths: string[];
	    watchedRegistry: string[];
	    fileEvents: WatchEvent[];
	    registryEvents: WatchEvent[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new MonitorState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.startedAt = source["startedAt"];
	        this.watchedPaths = source["watchedPaths"];
	        this.watchedRegistry = source["watchedRegistry"];
	        this.fileEvents = this.convertValues(source["fileEvents"], WatchEvent);
	        this.registryEvents = this.convertValues(source["registryEvents"], WatchEvent);
	        this.warnings = source["warnings"];
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
	export class Highlight {
	    title: string;
	    level: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new Highlight(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.level = source["level"];
	        this.detail = source["detail"];
	    }
	}
	export class AutorunEntry {
	    scope: string;
	    location: string;
	    name: string;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new AutorunEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.location = source["location"];
	        this.name = source["name"];
	        this.command = source["command"];
	    }
	}
	export class ConnectionInfo {
	    protocol: string;
	    status: string;
	    localAddr: string;
	    remoteAddr: string;
	    pid: number;
	    processName: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocol = source["protocol"];
	        this.status = source["status"];
	        this.localAddr = source["localAddr"];
	        this.remoteAddr = source["remoteAddr"];
	        this.pid = source["pid"];
	        this.processName = source["processName"];
	    }
	}
	export class ServiceInfo {
	    name: string;
	    displayName: string;
	    state: string;
	    startType: string;
	
	    static createFrom(source: any = {}) {
	        return new ServiceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.state = source["state"];
	        this.startType = source["startType"];
	    }
	}
	export class ProcessTreeNode {
	    pid: number;
	    parentPid: number;
	    depth: number;
	    name: string;
	    path: string;
	    status: string;
	    threads: number;
	    cpuPercent: number;
	    memoryMB: number;
	    hasChildren: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProcessTreeNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.parentPid = source["parentPid"];
	        this.depth = source["depth"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.status = source["status"];
	        this.threads = source["threads"];
	        this.cpuPercent = source["cpuPercent"];
	        this.memoryMB = source["memoryMB"];
	        this.hasChildren = source["hasChildren"];
	    }
	}
	export class ProcessInfo {
	    pid: number;
	    parentPid: number;
	    parentName: string;
	    name: string;
	    path: string;
	    commandLine: string;
	    status: string;
	    threads: number;
	    cpuPercent: number;
	    memoryMB: number;
	
	    static createFrom(source: any = {}) {
	        return new ProcessInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.parentPid = source["parentPid"];
	        this.parentName = source["parentName"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.commandLine = source["commandLine"];
	        this.status = source["status"];
	        this.threads = source["threads"];
	        this.cpuPercent = source["cpuPercent"];
	        this.memoryMB = source["memoryMB"];
	    }
	}
	export class DiskInfo {
	    path: string;
	    label: string;
	    fileSystem: string;
	    totalGB: number;
	    usedGB: number;
	    usedPct: number;
	
	    static createFrom(source: any = {}) {
	        return new DiskInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.label = source["label"];
	        this.fileSystem = source["fileSystem"];
	        this.totalGB = source["totalGB"];
	        this.usedGB = source["usedGB"];
	        this.usedPct = source["usedPct"];
	    }
	}
	export class Overview {
	    hostname: string;
	    platform: string;
	    platformVersion: string;
	    kernelVersion: string;
	    architecture: string;
	    bootTime: string;
	    uptime: string;
	    cpuModel: string;
	    logicalCores: number;
	    cpuLoad: number;
	    memoryTotalGB: number;
	    memoryUsedGB: number;
	    memoryUsedPct: number;
	    swapUsedGB: number;
	    swapTotalGB: number;
	    receivedGB: number;
	    sentGB: number;
	    disks: DiskInfo[];
	    processCount: number;
	    serviceCount: number;
	    connectionCount: number;
	    autorunCount: number;
	
	    static createFrom(source: any = {}) {
	        return new Overview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hostname = source["hostname"];
	        this.platform = source["platform"];
	        this.platformVersion = source["platformVersion"];
	        this.kernelVersion = source["kernelVersion"];
	        this.architecture = source["architecture"];
	        this.bootTime = source["bootTime"];
	        this.uptime = source["uptime"];
	        this.cpuModel = source["cpuModel"];
	        this.logicalCores = source["logicalCores"];
	        this.cpuLoad = source["cpuLoad"];
	        this.memoryTotalGB = source["memoryTotalGB"];
	        this.memoryUsedGB = source["memoryUsedGB"];
	        this.memoryUsedPct = source["memoryUsedPct"];
	        this.swapUsedGB = source["swapUsedGB"];
	        this.swapTotalGB = source["swapTotalGB"];
	        this.receivedGB = source["receivedGB"];
	        this.sentGB = source["sentGB"];
	        this.disks = this.convertValues(source["disks"], DiskInfo);
	        this.processCount = source["processCount"];
	        this.serviceCount = source["serviceCount"];
	        this.connectionCount = source["connectionCount"];
	        this.autorunCount = source["autorunCount"];
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
	export class SystemSnapshot {
	    generatedAt: string;
	    overview: Overview;
	    processes: ProcessInfo[];
	    processTree: ProcessTreeNode[];
	    services: ServiceInfo[];
	    connections: ConnectionInfo[];
	    autoruns: AutorunEntry[];
	    highlights: Highlight[];
	    monitor: MonitorState;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new SystemSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.generatedAt = source["generatedAt"];
	        this.overview = this.convertValues(source["overview"], Overview);
	        this.processes = this.convertValues(source["processes"], ProcessInfo);
	        this.processTree = this.convertValues(source["processTree"], ProcessTreeNode);
	        this.services = this.convertValues(source["services"], ServiceInfo);
	        this.connections = this.convertValues(source["connections"], ConnectionInfo);
	        this.autoruns = this.convertValues(source["autoruns"], AutorunEntry);
	        this.highlights = this.convertValues(source["highlights"], Highlight);
	        this.monitor = this.convertValues(source["monitor"], MonitorState);
	        this.warnings = source["warnings"];
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
	export class AnalysisBundle {
	    version: string;
	    host: string;
	    generatedAt: string;
	    snapshot: SystemSnapshot;
	    inventory: InventorySnapshot;
	
	    static createFrom(source: any = {}) {
	        return new AnalysisBundle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.host = source["host"];
	        this.generatedAt = source["generatedAt"];
	        this.snapshot = this.convertValues(source["snapshot"], SystemSnapshot);
	        this.inventory = this.convertValues(source["inventory"], InventorySnapshot);
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
	
	
	
	
	
	
	export class ModuleInfo {
	    name: string;
	    path: string;
	    baseAddress: string;
	    sizeKB: number;
	
	    static createFrom(source: any = {}) {
	        return new ModuleInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.baseAddress = source["baseAddress"];
	        this.sizeKB = source["sizeKB"];
	    }
	}
	
	
	export class ThreadInfo {
	    threadId: number;
	    ownerPid: number;
	    basePriority: number;
	
	    static createFrom(source: any = {}) {
	        return new ThreadInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.threadId = source["threadId"];
	        this.ownerPid = source["ownerPid"];
	        this.basePriority = source["basePriority"];
	    }
	}
	export class ProcessRef {
	    pid: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ProcessRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.name = source["name"];
	    }
	}
	export class ProcessDetail {
	    process: ProcessInfo;
	    parentChain: ProcessRef[];
	    children: ProcessRef[];
	    threads: ThreadInfo[];
	    modules: ModuleInfo[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new ProcessDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.process = this.convertValues(source["process"], ProcessInfo);
	        this.parentChain = this.convertValues(source["parentChain"], ProcessRef);
	        this.children = this.convertValues(source["children"], ProcessRef);
	        this.threads = this.convertValues(source["threads"], ThreadInfo);
	        this.modules = this.convertValues(source["modules"], ModuleInfo);
	        this.warnings = source["warnings"];
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
	
	
	
	
	
	
	
	export class UISettings {
	    windowMode: string;
	
	    static createFrom(source: any = {}) {
	        return new UISettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.windowMode = source["windowMode"];
	    }
	}

}

