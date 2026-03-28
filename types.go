package main

type SystemSnapshot struct {
	GeneratedAt string            `json:"generatedAt"`
	Overview    Overview          `json:"overview"`
	Processes   []ProcessInfo     `json:"processes"`
	ProcessTree []ProcessTreeNode `json:"processTree"`
	Services    []ServiceInfo     `json:"services"`
	Connections []ConnectionInfo  `json:"connections"`
	Autoruns    []AutorunEntry    `json:"autoruns"`
	Highlights  []Highlight       `json:"highlights"`
	Monitor     MonitorState      `json:"monitor"`
	Warnings    []string          `json:"warnings"`
}

type InventorySnapshot struct {
	GeneratedAt string              `json:"generatedAt"`
	Drivers     []DriverInfo        `json:"drivers"`
	Tasks       []ScheduledTaskInfo `json:"tasks"`
	Warnings    []string            `json:"warnings"`
}

type AnalysisBundle struct {
	Version     string            `json:"version"`
	Host        string            `json:"host"`
	GeneratedAt string            `json:"generatedAt"`
	Snapshot    SystemSnapshot    `json:"snapshot"`
	Inventory   InventorySnapshot `json:"inventory"`
}

type Overview struct {
	Hostname        string     `json:"hostname"`
	Platform        string     `json:"platform"`
	PlatformVersion string     `json:"platformVersion"`
	KernelVersion   string     `json:"kernelVersion"`
	Architecture    string     `json:"architecture"`
	BootTime        string     `json:"bootTime"`
	Uptime          string     `json:"uptime"`
	CPUModel        string     `json:"cpuModel"`
	LogicalCores    int        `json:"logicalCores"`
	CPULoad         float64    `json:"cpuLoad"`
	MemoryTotalGB   float64    `json:"memoryTotalGB"`
	MemoryUsedGB    float64    `json:"memoryUsedGB"`
	MemoryUsedPct   float64    `json:"memoryUsedPct"`
	SwapUsedGB      float64    `json:"swapUsedGB"`
	SwapTotalGB     float64    `json:"swapTotalGB"`
	ReceivedGB      float64    `json:"receivedGB"`
	SentGB          float64    `json:"sentGB"`
	Disks           []DiskInfo `json:"disks"`
	ProcessCount    int        `json:"processCount"`
	ServiceCount    int        `json:"serviceCount"`
	ConnectionCount int        `json:"connectionCount"`
	AutorunCount    int        `json:"autorunCount"`
}

type DiskInfo struct {
	Path       string  `json:"path"`
	Label      string  `json:"label"`
	FileSystem string  `json:"fileSystem"`
	TotalGB    float64 `json:"totalGB"`
	UsedGB     float64 `json:"usedGB"`
	UsedPct    float64 `json:"usedPct"`
}

type ProcessInfo struct {
	PID         int32   `json:"pid"`
	ParentPID   int32   `json:"parentPid"`
	ParentName  string  `json:"parentName"`
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	CommandLine string  `json:"commandLine"`
	Status      string  `json:"status"`
	Threads     int32   `json:"threads"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryMB    float64 `json:"memoryMB"`
}

type ProcessTreeNode struct {
	PID         int32   `json:"pid"`
	ParentPID   int32   `json:"parentPid"`
	Depth       int     `json:"depth"`
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	Status      string  `json:"status"`
	Threads     int32   `json:"threads"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryMB    float64 `json:"memoryMB"`
	HasChildren bool    `json:"hasChildren"`
}

type ProcessRef struct {
	PID  int32  `json:"pid"`
	Name string `json:"name"`
}

type ProcessDetail struct {
	Process     ProcessInfo  `json:"process"`
	ParentChain []ProcessRef `json:"parentChain"`
	Children    []ProcessRef `json:"children"`
	Threads     []ThreadInfo `json:"threads"`
	Modules     []ModuleInfo `json:"modules"`
	Warnings    []string     `json:"warnings"`
}

type ThreadInfo struct {
	ThreadID     uint32 `json:"threadId"`
	OwnerPID     int32  `json:"ownerPid"`
	BasePriority int32  `json:"basePriority"`
}

type ModuleInfo struct {
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	BaseAddress string  `json:"baseAddress"`
	SizeKB      float64 `json:"sizeKB"`
}

type ServiceInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	State       string `json:"state"`
	StartType   string `json:"startType"`
}

type DriverInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	State       string `json:"state"`
	StartMode   string `json:"startMode"`
	Path        string `json:"path"`
	ServiceType string `json:"serviceType"`
}

type ScheduledTaskInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	State       string `json:"state"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Command     string `json:"command"`
}

type ConnectionInfo struct {
	Protocol    string `json:"protocol"`
	Status      string `json:"status"`
	LocalAddr   string `json:"localAddr"`
	RemoteAddr  string `json:"remoteAddr"`
	PID         int32  `json:"pid"`
	ProcessName string `json:"processName"`
}

type AutorunEntry struct {
	Scope    string `json:"scope"`
	Location string `json:"location"`
	Name     string `json:"name"`
	Command  string `json:"command"`
}

type Highlight struct {
	Title  string `json:"title"`
	Level  string `json:"level"`
	Detail string `json:"detail"`
}

type MonitorState struct {
	StartedAt       string       `json:"startedAt"`
	WatchedPaths    []string     `json:"watchedPaths"`
	WatchedRegistry []string     `json:"watchedRegistry"`
	FileEvents      []WatchEvent `json:"fileEvents"`
	RegistryEvents  []WatchEvent `json:"registryEvents"`
	Warnings        []string     `json:"warnings"`
}

type WatchEvent struct {
	Time   string `json:"time"`
	Source string `json:"source"`
	Action string `json:"action"`
	Target string `json:"target"`
	Detail string `json:"detail"`
}
