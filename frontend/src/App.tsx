import React, {
  Component,
  startTransition,
  useDeferredValue,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type Dispatch,
  type ReactNode,
  type SetStateAction,
} from "react";
import "./App.css";
import {
  ControlService,
  DisableAutorun,
  ExportCurrentBundle,
  GetInventory,
  GetProcessDetail,
  GetSnapshot,
  GetUISettings,
  ImportBundleDialog,
  KillProcess,
  SetWindowMode,
  UpdateServiceStartType,
} from "../wailsjs/go/main/App";
import { main } from "../wailsjs/go/models";

type TabId = "overview" | "process" | "service" | "driver" | "task" | "network" | "autorun" | "monitor";
type SearchState = Record<TabId, string>;
type Tone = "neutral" | "good" | "attention" | "warning";

const tabs: Array<{ id: TabId; title: string; subtitle: string }> = [
  { id: "overview", title: "总览", subtitle: "整机状态与重点提示" },
  { id: "process", title: "进程", subtitle: "父子关系树、线程与模块" },
  { id: "service", title: "服务", subtitle: "服务状态与启动方式" },
  { id: "driver", title: "驱动", subtitle: "驱动状态与路径" },
  { id: "task", title: "计划任务", subtitle: "任务状态与执行命令" },
  { id: "network", title: "网络", subtitle: "连接与进程映射" },
  { id: "autorun", title: "启动项", subtitle: "Run 键与启动目录" },
  { id: "monitor", title: "监控", subtitle: "文件与注册表实时事件" },
];

const defaultSearches: SearchState = {
  overview: "",
  process: "",
  service: "",
  driver: "",
  task: "",
  network: "",
  autorun: "",
  monitor: "",
};

function App() {
  const [activeTab, setActiveTab] = useState<TabId>("overview");
  const [snapshot, setSnapshot] = useState<main.SystemSnapshot | null>(null);
  const [inventory, setInventory] = useState<main.InventorySnapshot | null>(null);
  const [processDetail, setProcessDetail] = useState<main.ProcessDetail | null>(null);
  const [selectedPid, setSelectedPid] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [inventoryRefreshing, setInventoryRefreshing] = useState(false);
  const [detailRefreshing, setDetailRefreshing] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [windowMode, setWindowMode] = useState("half");
  const [dataMode, setDataMode] = useState<"live" | "imported">("live");
  const [actionKey, setActionKey] = useState("");
  const [searches, setSearches] = useState<SearchState>(defaultSearches);
  const [monitorPageSize, setMonitorPageSize] = useState(50);
  const [monitorPage, setMonitorPage] = useState(1);
  const snapshotBusyRef = useRef(false);
  const inventoryBusyRef = useRef(false);
  const detailBusyRef = useRef(false);

  const processSearch = useDeferredValue(searches.process.trim().toLowerCase());
  const serviceSearch = useDeferredValue(searches.service.trim().toLowerCase());
  const driverSearch = useDeferredValue(searches.driver.trim().toLowerCase());
  const taskSearch = useDeferredValue(searches.task.trim().toLowerCase());
  const networkSearch = useDeferredValue(searches.network.trim().toLowerCase());
  const autorunSearch = useDeferredValue(searches.autorun.trim().toLowerCase());
  const monitorSearch = useDeferredValue(searches.monitor.trim().toLowerCase());

  async function loadSnapshot(mode: "initial" | "manual" | "auto") {
    if (snapshotBusyRef.current) return;
    snapshotBusyRef.current = true;
    if (mode === "initial") setLoading(true);
    else setRefreshing(true);
    setError("");
    try {
      const result = await GetSnapshot();
      startTransition(() => {
        setSnapshot(normalizeSnapshot(result));
        setDataMode("live");
      });
    } catch (loadError) {
      setError(extractError(loadError));
    } finally {
      snapshotBusyRef.current = false;
      setLoading(false);
      setRefreshing(false);
    }
  }

  async function loadInventory(mode: "initial" | "manual" | "auto") {
    if (inventoryBusyRef.current) return;
    inventoryBusyRef.current = true;
    if (mode !== "initial") setInventoryRefreshing(true);
    try {
      const result = await GetInventory();
      startTransition(() => {
        setInventory(normalizeInventory(result));
        setDataMode("live");
      });
    } catch (loadError) {
      setError(extractError(loadError));
    } finally {
      inventoryBusyRef.current = false;
      setInventoryRefreshing(false);
    }
  }

  async function loadProcessDetail(pid: number) {
    if (!pid || detailBusyRef.current) return;
    detailBusyRef.current = true;
    setDetailRefreshing(true);
    try {
      const result = await GetProcessDetail(pid);
      startTransition(() => setProcessDetail(normalizeProcessDetail(result)));
    } catch (loadError) {
      setError(extractError(loadError));
      setProcessDetail(null);
    } finally {
      detailBusyRef.current = false;
      setDetailRefreshing(false);
    }
  }

  async function loadUISettings() {
    try {
      const settings = await GetUISettings();
      setWindowMode(settings.windowMode || "half");
    } catch (loadError) {
      setError(extractError(loadError));
    }
  }

  async function applyWindowModeChange(mode: string) {
    try {
      const settings = await SetWindowMode(mode);
      setWindowMode(settings.windowMode || mode);
      setSettingsOpen(false);
    } catch (saveError) {
      setError(extractError(saveError));
    }
  }

  async function exportBundle() {
    try {
      const path = await ExportCurrentBundle();
      setNotice(`分析包已导出：${path}`);
    } catch (exportError) {
      setError(extractError(exportError));
    }
  }

  async function importBundle() {
    try {
      const bundle = main.AnalysisBundle.createFrom(await ImportBundleDialog());
      startTransition(() => {
        setSnapshot(normalizeSnapshot(bundle.snapshot));
        setInventory(normalizeInventory(bundle.inventory));
        setDataMode("imported");
        setActiveTab("overview");
      });
      setNotice(`已导入分析包：${bundle.host} / ${formatDateTime(bundle.generatedAt)}`);
    } catch (importError) {
      const message = extractError(importError);
      if (!message.includes("no bundle selected")) {
        setError(message);
      }
    }
  }

  async function handleKillProcess(pid: number) {
    setActionKey(`process-${pid}`);
    try {
      await KillProcess(pid);
      setNotice(`已结束进程 PID ${pid}`);
      await loadSnapshot("manual");
      setProcessDetail(null);
    } catch (killError) {
      setError(extractError(killError));
    } finally {
      setActionKey("");
    }
  }

  async function handleDisableAutorun(item: main.AutorunEntry) {
    setActionKey(`autorun-${item.scope}-${item.name}`);
    try {
      await DisableAutorun(item.scope, item.location, item.name);
      setNotice(`已禁用启动项：${item.name}`);
      await loadSnapshot("manual");
    } catch (disableError) {
      setError(extractError(disableError));
    } finally {
      setActionKey("");
    }
  }

  async function handleServiceAction(name: string, action: string) {
    setActionKey(`service-${name}-${action}`);
    try {
      await ControlService(name, action);
      setNotice(`服务 ${name} 已执行：${serviceActionLabel(action)}`);
      await loadSnapshot("manual");
    } catch (serviceError) {
      setError(extractError(serviceError));
    } finally {
      setActionKey("");
    }
  }

  async function handleServiceStartType(name: string, startType: string) {
    setActionKey(`service-type-${name}`);
    try {
      await UpdateServiceStartType(name, startType);
      setNotice(`服务 ${name} 启动类型已修改为：${startType}`);
      await loadSnapshot("manual");
    } catch (serviceError) {
      setError(extractError(serviceError));
    } finally {
      setActionKey("");
    }
  }

  useEffect(() => {
    void Promise.allSettled([loadSnapshot("initial"), loadInventory("initial"), loadUISettings()]);
  }, []);

  useEffect(() => {
    if (!autoRefresh || dataMode !== "live") return;
    const timer = window.setInterval(() => void loadSnapshot("auto"), 8000);
    return () => window.clearInterval(timer);
  }, [autoRefresh, dataMode]);

  useEffect(() => {
    if ((activeTab === "driver" || activeTab === "task") && !inventory) {
      void loadInventory("manual");
    }
  }, [activeTab, inventory]);

  useEffect(() => {
    const processTree = ensureArray(snapshot?.processTree);
    if (!processTree.length) {
      setSelectedPid(null);
      return;
    }
    if (selectedPid === null || !processTree.some((item) => item.pid === selectedPid)) {
      setSelectedPid(processTree[0].pid);
    }
  }, [selectedPid, snapshot?.processTree]);

  useEffect(() => {
    if (selectedPid !== null) void loadProcessDetail(selectedPid);
  }, [selectedPid]);

  const overview = snapshot?.overview;
  const filteredProcessTree = useMemo(() => ensureArray(snapshot?.processTree).filter((item) => matches(processSearch, [String(item.pid), item.name, item.path, item.status])), [processSearch, snapshot?.processTree]);
  const filteredServices = useMemo(() => ensureArray(snapshot?.services).filter((item) => matches(serviceSearch, [item.name, item.displayName, item.state, item.startType])), [serviceSearch, snapshot?.services]);
  const filteredDrivers = useMemo(() => ensureArray(inventory?.drivers).filter((item) => matches(driverSearch, [item.name, item.displayName, item.state, item.startMode, item.path, item.serviceType])), [driverSearch, inventory?.drivers]);
  const filteredTasks = useMemo(() => ensureArray(inventory?.tasks).filter((item) => matches(taskSearch, [item.name, item.path, item.state, item.author, item.description, item.command])), [inventory?.tasks, taskSearch]);
  const filteredConnections = useMemo(() => ensureArray(snapshot?.connections).filter((item) => matches(networkSearch, [item.protocol, item.status, item.localAddr, item.remoteAddr, String(item.pid), item.processName])), [networkSearch, snapshot?.connections]);
  const filteredAutoruns = useMemo(() => ensureArray(snapshot?.autoruns).filter((item) => matches(autorunSearch, [item.scope, item.location, item.name, item.command])), [autorunSearch, snapshot?.autoruns]);
  const filteredMonitorEvents = useMemo(() => [...ensureArray(snapshot?.monitor?.registryEvents), ...ensureArray(snapshot?.monitor?.fileEvents)].filter((item) => matches(monitorSearch, [item.source, item.action, item.target, item.detail])).sort((left, right) => (left.time < right.time ? 1 : -1)), [monitorSearch, snapshot?.monitor]);
  const selectedTreeNode = ensureArray(snapshot?.processTree).find((item) => item.pid === selectedPid) ?? null;
  const monitorTotalPages = Math.max(1, Math.ceil(filteredMonitorEvents.length / monitorPageSize));
  const pagedMonitorEvents = useMemo(() => {
    const start = (monitorPage - 1) * monitorPageSize;
    return filteredMonitorEvents.slice(start, start + monitorPageSize);
  }, [filteredMonitorEvents, monitorPage, monitorPageSize]);

  useEffect(() => {
    setMonitorPage(1);
  }, [monitorPageSize, monitorSearch]);

  useEffect(() => {
    setMonitorPage((current) => Math.min(current, monitorTotalPages));
  }, [monitorTotalPages]);

  return (
    <ErrorBoundary>
      <div className="shell">
        <aside className="sidebar">
          <div className="sidebar-metrics">
            <SidebarMetricCard accent="cpu" label="CPU 负载" value={formatPercent(overview?.cpuLoad)} />
            <SidebarMetricCard accent="memory" label="内存占用" value={formatPercent(overview?.memoryUsedPct)} />
            <SidebarMetricCard accent="network" label="网络连接" value={formatCount(overview?.connectionCount)} />
            <SidebarMetricCard accent="launch" label="启动项" value={formatCount(overview?.autorunCount)} />
          </div>
          <div className="sidebar-quick-list">
            <SidebarQuickItem label="进程" value={formatCount(overview?.processCount)} />
            <SidebarQuickItem label="服务" value={formatCount(overview?.serviceCount)} />
            <SidebarQuickItem label="连接" value={formatCount(overview?.connectionCount)} />
            <SidebarQuickItem label="运行" value={overview?.uptime || "-"} />
          </div>
          <div className="sidebar-footer">
            <div className="sidebar-stat"><span>主机</span><strong>{overview?.hostname ?? "正在等待快照"}</strong></div>
            <div className="sidebar-stat"><span>系统</span><strong>{overview ? `${overview.platform} ${overview.platformVersion}` : "正在等待快照"}</strong></div>
            <div className="sidebar-stat"><span>上次刷新</span><strong>{formatDateTime(snapshot?.generatedAt)}</strong></div>
          </div>
        </aside>
        <main className="workspace">
          <header className="topbar">
            <div className="topbar-actions">
              <button className="icon-button" onClick={() => setSettingsOpen((value) => !value)} title="界面设置" type="button">⚙</button>
              <button className={`icon-button ${(refreshing || inventoryRefreshing) ? "is-spinning" : ""}`} disabled={loading || refreshing || inventoryRefreshing} onClick={() => void Promise.allSettled([loadSnapshot("manual"), loadInventory("manual")])} title="立即刷新" type="button">↻</button>
            </div>
          </header>
          <nav className="top-tabs">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                className={`top-tab-button ${activeTab === tab.id ? "is-active" : ""}`}
                onClick={() => setActiveTab(tab.id)}
                type="button"
              >
                <strong>{tab.title}</strong>
                <span>{tabBadge(tab.id, snapshot, inventory)}</span>
              </button>
            ))}
          </nav>
          {settingsOpen ? (
            <div className="settings-overlay" onClick={() => setSettingsOpen(false)} role="presentation">
              <div className="settings-modal" onClick={(event) => event.stopPropagation()}>
                <div className="settings-head">
                  <strong>界面尺寸</strong>
                  <button className="settings-close" onClick={() => setSettingsOpen(false)} type="button">
                    关闭
                  </button>
                </div>
                <p className="settings-copy">设置后会自动保存，下次打开仍按此模式启动。</p>
                <div className="settings-options">
                  <button className={`settings-option ${windowMode === "standard" ? "is-active" : ""}`} onClick={() => void applyWindowModeChange("standard")} type="button">标准窗口</button>
                  <button className={`settings-option ${windowMode === "half" ? "is-active" : ""}`} onClick={() => void applyWindowModeChange("half")} type="button">半屏窗口</button>
                  <button className={`settings-option ${windowMode === "fullscreen" ? "is-active" : ""}`} onClick={() => void applyWindowModeChange("fullscreen")} type="button">全屏</button>
                </div>
                <div className="settings-tools">
                  <button className="settings-option" onClick={() => void exportBundle()} type="button">导出分析包</button>
                  <button className="settings-option" onClick={() => void importBundle()} type="button">导入分析包</button>
                </div>
              </div>
            </div>
          ) : null}
          {error ? <Banner tone="warning" text={error} /> : null}
          {notice ? <Banner tone="good" text={notice} /> : null}
          {snapshot?.warnings?.length ? <Banner tone="attention" text={`快照采集受限：${snapshot.warnings.join("；")}`} /> : null}
          {inventory?.warnings?.length ? <Banner tone="attention" text={`驱动或任务采集受限：${inventory.warnings.join("；")}`} /> : null}
          {snapshot?.monitor?.warnings?.length ? <Banner tone="attention" text={`实时监控受限：${snapshot.monitor.warnings.join("；")}`} /> : null}
          {loading && !snapshot ? <div className="loading-state"><div className="spinner" /><div><h3>正在采集首帧快照</h3><p>首次加载会慢一点，后续会自动刷新。</p></div></div> : null}
          {snapshot ? (
            <div className="content-scroll">
              {activeTab === "overview" ? (
                <div className="overview-layout">
                  <div className="stack">
                    <Panel title="重点提示" subtitle="根据当前快照自动生成">
                      <div className="highlight-list">
                        {ensureArray(snapshot.highlights).map((item) => (
                          <div className={`highlight-card tone-${toneFromLevel(item.level)}`} key={item.title}>
                            <div className="highlight-head"><StatusPill label={toneLabel(item.level)} tone={toneFromLevel(item.level)} /><strong>{item.title}</strong></div>
                            <p>{item.detail}</p>
                          </div>
                        ))}
                      </div>
                    </Panel>
                    <Panel title="磁盘占用" subtitle="快速判断磁盘压力点">
                      <div className="disk-list">
                        {ensureArray(overview?.disks).length ? ensureArray(overview?.disks).map((disk) => (
                          <div className="disk-row" key={disk.path}>
                            <div className="disk-head"><div><strong>{disk.label}</strong><span>{disk.fileSystem || "Unknown"} · {formatNumber(disk.usedGB)} / {formatNumber(disk.totalGB)} GB</span></div><strong>{formatPercent(disk.usedPct)}</strong></div>
                            <div className="progress-track"><span className="progress-fill" style={{ width: `${Math.min(disk.usedPct, 100)}%` }} /></div>
                          </div>
                        )) : <EmptyState text="未获取到磁盘信息。" />}
                      </div>
                    </Panel>
                  </div>
                  <div className="stack">
                    <Panel title="系统画像" subtitle="一眼确认当前主机身份">
                      <div className="facts-grid">
                        <Fact label="主机名" value={overview?.hostname} />
                        <Fact label="系统版本" value={overview ? `${overview.platform} ${overview.platformVersion}` : undefined} />
                        <Fact label="内核版本" value={overview?.kernelVersion} />
                        <Fact label="系统架构" value={overview?.architecture} />
                        <Fact label="开机时间" value={overview?.bootTime} />
                        <Fact label="已运行" value={overview?.uptime} />
                        <Fact label="交换区" value={`${formatNumber(overview?.swapUsedGB)} / ${formatNumber(overview?.swapTotalGB)} GB`} />
                      </div>
                    </Panel>
                    <Panel title="高占用进程" subtitle="单击切到进程页继续展开">
                      <div className="mini-table">
                        <div className="mini-table-head"><span>进程</span><span>PID</span><span>内存</span><span>CPU</span></div>
                        {ensureArray(snapshot.processes).slice(0, 8).map((item) => (
                          <button key={item.pid} className="mini-table-row is-button" onClick={() => { setActiveTab("process"); setSelectedPid(item.pid); }} type="button">
                            <div className="cell-main"><strong>{item.name}</strong><span>{truncate(item.path || item.commandLine, 72)}</span></div>
                            <span>{item.pid}</span><span>{formatNumber(item.memoryMB)} MB</span><span>{formatPercent(item.cpuPercent)}</span>
                          </button>
                        ))}
                      </div>
                    </Panel>
                  </div>
                </div>
              ) : null}

              {activeTab === "process" ? (
                <div className="process-layout">
                  <Panel title={`进程树 (${filteredProcessTree.length})`} subtitle="与详情联动，按名称、PID、路径筛选" toolbar={<SearchBox value={searches.process} onChange={(value) => updateSearch(setSearches, "process", value)} placeholder="筛选进程..." />}>
                    <div className="tree-list">
                      {filteredProcessTree.length ? filteredProcessTree.map((item) => (
                        <button key={`${item.pid}-${item.parentPid}`} className={`tree-row ${selectedPid === item.pid ? "is-selected" : ""}`} onClick={() => setSelectedPid(item.pid)} style={treeRowStyle(item.depth)} type="button">
                          <div className="tree-indent" style={{ width: `${item.depth * 18}px` }} />
                          <div className="tree-main"><strong>{item.name}</strong><span>PID {item.pid}</span></div>
                          <div className="tree-meta"><span>{formatNumber(item.memoryMB)} MB</span><span>{formatPercent(item.cpuPercent)}</span></div>
                        </button>
                      )) : <EmptyState text="没有匹配的进程节点。" />}
                    </div>
                  </Panel>
                  <Panel title={selectedTreeNode ? `${selectedTreeNode.name} / PID ${selectedTreeNode.pid}` : "进程详情"} subtitle="线程、模块、父链与子进程" toolbar={<div className="toolbar-cluster">{detailRefreshing ? <span className="soft-badge">详情刷新中</span> : null}{processDetail ? <button className="inline-action" disabled={actionKey === `process-${processDetail.process.pid}`} onClick={() => void handleKillProcess(processDetail.process.pid)} type="button">结束进程</button> : null}</div>}>
                    {processDetail ? (
                      <>
                        <div className="facts-grid detail-grid">
                          <Fact label="父进程" value={processDetail.process.parentName || "-"} />
                          <Fact label="状态" value={processDetail.process.status} />
                          <Fact label="线程数" value={String(processDetail.process.threads)} />
                          <Fact label="CPU" value={formatPercent(processDetail.process.cpuPercent)} />
                          <Fact label="内存" value={`${formatNumber(processDetail.process.memoryMB)} MB`} />
                          <Fact label="路径" value={processDetail.process.path || "-"} />
                        </div>
                        <div className="detail-block">
                          <h4>父子关系</h4>
                          <div className="relation-group">
                            <span className="relation-label">父链</span>
                            <ProcessFlow items={ensureArray(processDetail.parentChain)} emptyText="无" />
                          </div>
                          <div className="relation-group">
                            <span className="relation-label">子进程</span>
                            <ProcessChipList items={ensureArray(processDetail.children)} emptyText="无" />
                          </div>
                          <p className="detail-text mono">{processDetail.process.commandLine || "-"}</p>
                        </div>
                        <div className="detail-split">
                          <div>
                            <h4>线程</h4>
                            <div className="table-wrap"><table className="data-table compact"><thead><tr><th>TID</th><th>所属 PID</th><th>基础优先级</th></tr></thead><tbody>{ensureArray(processDetail.threads).map((item) => <tr key={item.threadId}><td className="mono">{item.threadId}</td><td>{item.ownerPid}</td><td>{item.basePriority}</td></tr>)}</tbody></table></div>
                          </div>
                          <div>
                            <h4>模块</h4>
                            <div className="table-wrap"><table className="data-table compact"><thead><tr><th>模块</th><th>基址</th><th>大小</th></tr></thead><tbody>{ensureArray(processDetail.modules).map((item) => <tr key={`${item.name}-${item.baseAddress}`}><td><div className="cell-main"><strong>{item.name}</strong><span>{truncate(item.path, 88)}</span></div></td><td className="mono">{item.baseAddress}</td><td>{formatNumber(item.sizeKB)} KB</td></tr>)}</tbody></table></div>
                          </div>
                        </div>
                        {ensureArray(processDetail.warnings).length ? <Banner tone="attention" text={processDetail.warnings.join("；")} /> : null}
                      </>
                    ) : <EmptyState text="选择上方进程节点后显示详情。" />}
                  </Panel>
                </div>
              ) : null}

              {activeTab === "service" ? <div className="single-panel-page"><DataPanel fill title={`服务 (${filteredServices.length})`} subtitle="查看服务状态、启动方式并进行基础控制" search={searches.service} onSearchChange={(value) => updateSearch(setSearches, "service", value)} placeholder="筛选服务..."><table className="data-table service-table"><thead><tr><th>服务名</th><th>显示名</th><th>状态</th><th>启动类型</th><th>操作</th></tr></thead><tbody>{filteredServices.map((item) => <tr key={item.name}><td className="mono wrap-cell">{item.name}</td><td className="wrap-cell">{item.displayName}</td><td><StatusPill label={serviceStateLabel(item.state)} tone={stateTone(item.state)} /></td><td><select className="row-select" defaultValue={item.startType} onChange={(event) => void handleServiceStartType(item.name, event.target.value)}><option value="自动">自动</option><option value="手动">手动</option><option value="禁用">禁用</option></select></td><td><div className="row-actions">{item.state === "Running" ? <button className="inline-action" disabled={actionKey === `service-${item.name}-stop`} onClick={() => void handleServiceAction(item.name, "stop")} type="button">停止</button> : <button className="inline-action" disabled={actionKey === `service-${item.name}-start`} onClick={() => void handleServiceAction(item.name, "start")} type="button">启动</button>}<button className="inline-action secondary" disabled={actionKey === `service-${item.name}-restart`} onClick={() => void handleServiceAction(item.name, "restart")} type="button">重启</button></div></td></tr>)}</tbody></table>{!filteredServices.length ? <EmptyState text="没有匹配的服务。" /> : null}</DataPanel></div> : null}
              {activeTab === "driver" ? <div className="single-panel-page"><DataPanel fill title={`驱动 (${filteredDrivers.length})`} subtitle="驱动状态、启动方式与镜像路径" search={searches.driver} onSearchChange={(value) => updateSearch(setSearches, "driver", value)} placeholder="筛选驱动..."><table className="data-table driver-table"><thead><tr><th>驱动</th><th>显示名</th><th>状态</th><th>启动方式</th><th>类型</th></tr></thead><tbody>{filteredDrivers.map((item) => <tr key={item.name}><td className="mono wrap-cell">{item.name}</td><td><div className="cell-main wrap"><strong>{item.displayName}</strong><span>{item.path}</span></div></td><td><StatusPill label={item.state} tone={stateTone(item.state)} /></td><td>{item.startMode}</td><td>{item.serviceType}</td></tr>)}</tbody></table>{!filteredDrivers.length ? <EmptyState text="没有匹配的驱动。" /> : null}</DataPanel></div> : null}
              {activeTab === "task" ? <div className="single-panel-page"><DataPanel fill title={`计划任务 (${filteredTasks.length})`} subtitle="任务名称、路径、描述与执行命令" search={searches.task} onSearchChange={(value) => updateSearch(setSearches, "task", value)} placeholder="筛选计划任务..."><table className="data-table task-table"><thead><tr><th>任务</th><th>状态</th><th>作者</th><th>命令</th></tr></thead><tbody>{filteredTasks.map((item) => <tr key={`${item.path}-${item.name}`}><td><div className="cell-main wrap"><strong>{item.name}</strong><span>{item.path}</span><span>{item.description || "-"}</span></div></td><td><StatusPill label={item.state} tone={stateTone(item.state)} /></td><td className="wrap-cell">{item.author || "-"}</td><td className="mono wrap-cell">{item.command || "-"}</td></tr>)}</tbody></table>{!filteredTasks.length ? <EmptyState text="没有匹配的计划任务。" /> : null}</DataPanel></div> : null}
              {activeTab === "network" ? <div className="single-panel-page"><DataPanel fill title={`网络连接 (${filteredConnections.length})`} subtitle="端口、状态与进程映射" search={searches.network} onSearchChange={(value) => updateSearch(setSearches, "network", value)} placeholder="筛选连接..."><table className="data-table"><thead><tr><th>协议</th><th>状态</th><th>本地地址</th><th>远端地址</th><th>进程</th></tr></thead><tbody>{filteredConnections.map((item, index) => <tr key={`${item.pid}-${item.localAddr}-${item.remoteAddr}-${index}`}><td><StatusPill label={item.protocol} tone="neutral" /></td><td><StatusPill label={item.status} tone={networkTone(item.status)} /></td><td className="mono">{item.localAddr}</td><td className="mono">{item.remoteAddr}</td><td><div className="cell-main"><strong>{item.processName}</strong><span>PID {item.pid}</span></div></td></tr>)}</tbody></table>{!filteredConnections.length ? <EmptyState text="没有匹配的连接。" /> : null}</DataPanel></div> : null}
              {activeTab === "autorun" ? <div className="single-panel-page"><DataPanel fill title={`启动项 (${filteredAutoruns.length})`} subtitle="Run 键、RunOnce 与启动目录，可直接禁用" search={searches.autorun} onSearchChange={(value) => updateSearch(setSearches, "autorun", value)} placeholder="筛选启动项..."><table className="data-table autorun-table"><thead><tr><th>名称</th><th>范围</th><th>位置</th><th>命令</th><th>操作</th></tr></thead><tbody>{filteredAutoruns.map((item) => <tr key={`${item.scope}-${item.location}-${item.name}`}><td className="wrap-cell">{item.name}</td><td><StatusPill label={item.scope} tone={item.scope === "Machine" ? "warning" : "attention"} /></td><td className="mono wrap-cell">{item.location}</td><td className="mono wrap-cell">{item.command}</td><td><button className="inline-action" disabled={actionKey === `autorun-${item.scope}-${item.name}`} onClick={() => void handleDisableAutorun(item)} type="button">禁用</button></td></tr>)}</tbody></table>{!filteredAutoruns.length ? <EmptyState text="没有匹配的启动项。" /> : null}</DataPanel></div> : null}
              {activeTab === "monitor" ? (
                <div className="monitor-layout">
                  <Panel title="监控范围" subtitle="当前优先监控高风险持久化位置和常见落地目录" toolbar={<span className="soft-badge">后台持续运行</span>}>
                    <div className="watch-grid">
                      <div className="watch-box"><h4>监控目录</h4>{ensureArray(snapshot.monitor?.watchedPaths).length ? ensureArray(snapshot.monitor?.watchedPaths).map((item) => <p className="watch-line mono" key={item}>{item}</p>) : <EmptyState text="当前没有有效的文件监控目录。" />}</div>
                      <div className="watch-box"><h4>监控注册表</h4>{ensureArray(snapshot.monitor?.watchedRegistry).length ? ensureArray(snapshot.monitor?.watchedRegistry).map((item) => <p className="watch-line mono" key={item}>{item}</p>) : <EmptyState text="当前没有有效的注册表监控项。" />}</div>
                    </div>
                  </Panel>
                  <DataPanel title={`事件流 (${filteredMonitorEvents.length})`} subtitle="默认每页 50 条，可切换查看更多" search={searches.monitor} onSearchChange={(value) => updateSearch(setSearches, "monitor", value)} placeholder="筛选事件..." toolbar={<div className="toolbar-cluster"><SearchBox value={searches.monitor} onChange={(value) => updateSearch(setSearches, "monitor", value)} placeholder="筛选事件..." /><select className="toolbar-select" value={monitorPageSize} onChange={(event) => setMonitorPageSize(Number(event.target.value))}><option value={50}>50 条</option><option value={100}>100 条</option><option value={200}>200 条</option></select><div className="pager"><button className="pager-button" disabled={monitorPage <= 1} onClick={() => setMonitorPage((value) => Math.max(1, value - 1))} type="button">上一页</button><span className="pager-status">{monitorPage} / {monitorTotalPages}</span><button className="pager-button" disabled={monitorPage >= monitorTotalPages} onClick={() => setMonitorPage((value) => Math.min(monitorTotalPages, value + 1))} type="button">下一页</button></div></div>}><table className="data-table"><thead><tr><th>时间</th><th>来源</th><th>动作</th><th>目标</th><th>详情</th></tr></thead><tbody>{pagedMonitorEvents.map((item, index) => <tr key={`${item.time}-${item.target}-${index}`}><td>{formatDateTime(item.time)}</td><td><StatusPill label={item.source} tone={item.source === "注册表" ? "warning" : "attention"} /></td><td>{item.action}</td><td className="mono wrap-cell">{item.target}</td><td className="mono wrap-cell">{item.detail}</td></tr>)}</tbody></table>{!pagedMonitorEvents.length ? <EmptyState text="监控事件列表为空。" /> : null}</DataPanel>
                </div>
              ) : null}
            </div>
          ) : null}
        </main>
      </div>
    </ErrorBoundary>
  );
}

type BoundaryProps = { children: ReactNode };
type BoundaryState = { error: string | null };

class ErrorBoundary extends Component<BoundaryProps, BoundaryState> {
  state: BoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): BoundaryState {
    return { error: error.message || "前端渲染发生异常" };
  }

  render() {
    if (this.state.error) {
      return <div className="fatal-shell"><div className="fatal-card"><h2>界面渲染被中断</h2><p>{this.state.error}</p><p>这次不会再直接掉成纯蓝页，把这段信息发给我就能继续定位。</p></div></div>;
    }
    return this.props.children;
  }
}

function SidebarMetricCard({ accent, label, value }: { accent: string; label: string; value: string }) {
  return <article className={`sidebar-metric-card accent-${accent}`}><span>{label}</span><strong>{value}</strong></article>;
}

function SidebarQuickItem({ label, value }: { label: string; value: string }) {
  return <div className="sidebar-quick-item"><span>{label}</span><strong>{value}</strong></div>;
}

function ProcessFlow({ items, emptyText }: { items: main.ProcessRef[]; emptyText: string }) {
  if (!items.length) {
    return <span className="relation-empty">{emptyText}</span>;
  }

  return (
    <div className="relation-flow">
      {items.map((item, index) => (
        <React.Fragment key={`${item.pid}-${item.name}`}>
          <span className="relation-chip">{item.name}({item.pid})</span>
          {index < items.length - 1 ? <span className="relation-arrow">→</span> : null}
        </React.Fragment>
      ))}
    </div>
  );
}

function ProcessChipList({ items, emptyText }: { items: main.ProcessRef[]; emptyText: string }) {
  if (!items.length) {
    return <span className="relation-empty">{emptyText}</span>;
  }

  return (
    <div className="relation-flow">
      {items.map((item) => (
        <span className="relation-chip" key={`${item.pid}-${item.name}`}>
          {item.name}({item.pid})
        </span>
      ))}
    </div>
  );
}

function Panel({ title, subtitle, toolbar, className, children }: { title: string; subtitle: string; toolbar?: ReactNode; className?: string; children: ReactNode }) {
  return <section className={`panel ${className ?? ""}`.trim()}><div className="panel-head"><div><h3>{title}</h3><p>{subtitle}</p></div>{toolbar}</div>{children}</section>;
}

function DataPanel({ title, subtitle, search, onSearchChange, placeholder, toolbar, fill, children }: { title: string; subtitle: string; search: string; onSearchChange: (value: string) => void; placeholder: string; toolbar?: ReactNode; fill?: boolean; children: ReactNode }) {
  return <Panel title={title} subtitle={subtitle} toolbar={toolbar ?? <SearchBox value={search} onChange={onSearchChange} placeholder={placeholder} />} className={fill ? "fill-panel" : undefined}><div className={`table-wrap ${fill ? "fill-table-wrap" : ""}`}>{children}</div></Panel>;
}

function SearchBox({ value, onChange, placeholder }: { value: string; onChange: (value: string) => void; placeholder: string }) {
  return <label className="search-box"><input value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} type="search" /></label>;
}

function Fact({ label, value }: { label: string; value?: string }) {
  return <div className="fact-card"><span>{label}</span><strong>{value || "-"}</strong></div>;
}

function Banner({ text, tone }: { text: string; tone: Tone }) {
  return <div className={`banner tone-${tone}`}>{text}</div>;
}

function EmptyState({ text }: { text: string }) {
  return <div className="empty-state">{text}</div>;
}

function StatusPill({ label, tone }: { label: string; tone: Tone }) {
  return <span className={`status-pill tone-${tone}`}>{label}</span>;
}

function ensureArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

function normalizeSnapshot(raw: main.SystemSnapshot) {
  const snapshot = main.SystemSnapshot.createFrom(raw);
  snapshot.processes = ensureArray(snapshot.processes);
  snapshot.processTree = ensureArray(snapshot.processTree);
  snapshot.services = ensureArray(snapshot.services);
  snapshot.connections = ensureArray(snapshot.connections);
  snapshot.autoruns = ensureArray(snapshot.autoruns);
  snapshot.highlights = ensureArray(snapshot.highlights);
  snapshot.warnings = ensureArray(snapshot.warnings);
  snapshot.monitor = main.MonitorState.createFrom(snapshot.monitor ?? { startedAt: "", watchedPaths: [], watchedRegistry: [], fileEvents: [], registryEvents: [], warnings: [] });
  snapshot.monitor.watchedPaths = ensureArray(snapshot.monitor.watchedPaths);
  snapshot.monitor.watchedRegistry = ensureArray(snapshot.monitor.watchedRegistry);
  snapshot.monitor.fileEvents = ensureArray(snapshot.monitor.fileEvents);
  snapshot.monitor.registryEvents = ensureArray(snapshot.monitor.registryEvents);
  snapshot.monitor.warnings = ensureArray(snapshot.monitor.warnings);
  snapshot.overview = main.Overview.createFrom(snapshot.overview ?? {});
  snapshot.overview.disks = ensureArray(snapshot.overview.disks);
  return snapshot;
}

function normalizeInventory(raw: main.InventorySnapshot) {
  const inventory = main.InventorySnapshot.createFrom(raw);
  inventory.drivers = ensureArray(inventory.drivers);
  inventory.tasks = ensureArray(inventory.tasks);
  inventory.warnings = ensureArray(inventory.warnings);
  return inventory;
}

function normalizeProcessDetail(raw: main.ProcessDetail) {
  const detail = main.ProcessDetail.createFrom(raw);
  detail.process = main.ProcessInfo.createFrom(detail.process ?? {});
  detail.parentChain = ensureArray(detail.parentChain);
  detail.children = ensureArray(detail.children);
  detail.threads = ensureArray(detail.threads);
  detail.modules = ensureArray(detail.modules);
  detail.warnings = ensureArray(detail.warnings);
  return detail;
}
function updateSearch(setter: Dispatch<SetStateAction<SearchState>>, key: TabId, value: string) {
  setter((current) => ({ ...current, [key]: value }));
}

function matches(query: string, fields: string[]) {
  if (!query) return true;
  return fields.some((field) => (field || "").toLowerCase().includes(query));
}

function formatPercent(value?: number) {
  return `${formatNumber(value)}%`;
}

function formatNumber(value?: number) {
  return (value ?? 0).toFixed(1);
}

function formatCount(value?: number) {
  return Intl.NumberFormat("zh-CN").format(value ?? 0);
}

function formatDateTime(value?: string) {
  if (!value) return "尚未刷新";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString("zh-CN", { hour12: false });
}

function extractError(error: unknown) {
  if (error instanceof Error) return error.message;
  return String(error);
}

function truncate(value: string, maxLength: number) {
  if (!value) return "-";
  if (value.length <= maxLength) return value;
  return `${value.slice(0, maxLength)}...`;
}

function stateTone(state: string): Tone {
  if (["Running", "Ready", "Enabled", "LISTEN", "LISTEN/IDLE"].includes(state)) return "good";
  if (["Stopped", "Disabled"].includes(state)) return "neutral";
  if (["ESTABLISHED"].includes(state)) return "attention";
  return "warning";
}

function serviceStateLabel(state: string) {
  switch (state) {
    case "Running":
      return "运行中";
    case "Stopped":
      return "已停止";
    case "Start Pending":
      return "启动中";
    case "Stop Pending":
      return "停止中";
    case "Paused":
      return "已暂停";
    default:
      return state;
  }
}

function serviceActionLabel(action: string) {
  switch (action) {
    case "start":
      return "启动";
    case "stop":
      return "停止";
    case "restart":
      return "重启";
    case "pause":
      return "暂停";
    case "continue":
      return "继续";
    default:
      return action;
  }
}

function networkTone(state: string): Tone {
  if (state === "ESTABLISHED") return "attention";
  if (state === "LISTEN" || state === "LISTEN/IDLE") return "good";
  if (state === "TIME_WAIT" || state === "CLOSE_WAIT") return "warning";
  return "neutral";
}

function toneFromLevel(level: string): Tone {
  switch (level) {
    case "warning":
      return "warning";
    case "attention":
      return "attention";
    default:
      return "good";
  }
}

function toneLabel(level: string) {
  switch (level) {
    case "warning":
      return "提醒";
    case "attention":
      return "关注";
    default:
      return "正常";
  }
}

function tabBadge(tab: TabId, snapshot: main.SystemSnapshot | null, inventory: main.InventorySnapshot | null) {
  switch (tab) {
    case "overview":
      return "Live";
    case "process":
      return formatCount(snapshot?.overview?.processCount);
    case "service":
      return formatCount(snapshot?.overview?.serviceCount);
    case "driver":
      return formatCount(ensureArray(inventory?.drivers).length);
    case "task":
      return formatCount(ensureArray(inventory?.tasks).length);
    case "network":
      return formatCount(snapshot?.overview?.connectionCount);
    case "autorun":
      return formatCount(snapshot?.overview?.autorunCount);
    case "monitor":
      return formatCount(ensureArray(snapshot?.monitor?.fileEvents).length + ensureArray(snapshot?.monitor?.registryEvents).length);
    default:
      return "0";
  }
}

function treeRowStyle(depth: number): CSSProperties {
  return {
    ["--tree-accent" as string]: treeAccent(depth),
  } as CSSProperties;
}

function treeAccent(depth: number) {
  const palette = ["#66c7ff", "#84e8b8", "#ffd479", "#ff9b6d", "#d1a8ff"];
  return palette[depth % palette.length];
}

export default App;
