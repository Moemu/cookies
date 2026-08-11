import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Check, FileText, RefreshCw, RotateCcw, Search, Upload, X } from "lucide-react";
import { useProject } from "../context/ProjectContext";
import {
  api,
  ApiRequestError,
  type ApiKnowledgeDocument,
  type ApiMiyunConnection,
  type ApiMiyunCrawlJob,
  type ApiMiyunHandoff,
  type ApiMiyunMaterial,
  type ApiMiyunMaterialSnapshot,
  type ApiMiyunProductProfile,
  type ApiMiyunProductSource,
  type ApiMiyunProfileQuery,
  type ApiProjectMediaAsset,
} from "../data/api";
import type { DataState } from "../types";
import { StateBoundary } from "./StateBoundary";

type View = "analysis" | "jobs" | "materials" | "fission";
type MaterialDetail = ApiMiyunMaterial & {
  snapshots: ApiMiyunMaterialSnapshot[];
};
type CardField =
  | "delivery_days"
  | "cumulative_impressions"
  | "related_ads"
  | "related_creators"
  | "material_score";
const viewMap: Record<string, View> = {
  产品分析: "analysis",
  采集任务: "jobs",
  素材候选: "materials",
  裂变任务: "fission",
};
const viewCopy: Record<
  View,
  { eyebrow: string; title: string; description: string }
> = {
  analysis: {
    eyebrow: "PRODUCT PROFILE",
    title: "产品分析",
    description: "汇集产品资料，生成并人工确认米云检索所需的产品画像。",
  },
  jobs: {
    eyebrow: "COLLECTION PROGRESS",
    title: "采集任务",
    description: "基于已确认的产品画像发起采集；运行中的任务会自动更新进度。",
  },
  materials: {
    eyebrow: "MATERIAL REVIEW",
    title: "素材候选",
    description: "按最新数据卡筛选候选素材，并记录每条素材的人工决策。",
  },
  fission: {
    eyebrow: "VERSIONED HANDOFF",
    title: "裂变任务",
    description: "将已确认素材与产品画像冻结成可追溯的裂变交接包。",
  },
};
const activeJobStatuses = new Set<ApiMiyunCrawlJob["status"]>([
  "queued",
  "running",
  "cooling_down",
]);
const activeMaterialStatuses = new Set<ApiMiyunMaterial["import_status"]>([
  "pending",
  "downloading",
]);
const materialPageSize = 8;
const previewConcurrency = 2;
const crawlContextVersion = 1;
const timeFormatter = new Intl.DateTimeFormat("zh-CN", {
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
});
const dateTimeFormatter = new Intl.DateTimeFormat("zh-CN", {
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
});
const formatSyncTime = (value: Date | null) =>
  value ? timeFormatter.format(value) : "等待首次同步";
const formatServerTime = (value?: string) =>
  value ? dateTimeFormatter.format(new Date(value)) : "暂无记录";
const jobStatusLabel: Record<ApiMiyunCrawlJob["status"], string> = {
  queued: "等待采集",
  running: "采集中",
  cooling_down: "等待冷却",
  auth_required: "需要重新授权",
  partial: "部分完成",
  succeeded: "已完成",
  failed: "采集失败",
  cancelled: "已取消",
};
const materialStatusLabel = {
  discovered: "待审核",
  confirmed: "已确认",
  rejected: "已拒绝",
} satisfies Record<ApiMiyunMaterial["selection_status"], string>;
const importStatusLabel = {
  pending: "等待入库",
  downloading: "正在入库",
  imported: "已入库",
  deduplicated: "已去重入库",
  failed: "入库失败",
  skipped: "已跳过",
} satisfies Record<ApiMiyunMaterial["import_status"], string>;
const materialTypeLabel: Record<string, string> = {
  product: "产品素材",
  video: "视频",
  image: "图片",
  text: "图文",
};
const listPreview = (values: string[], limit = 3) => {
  const visible = values.slice(0, limit);
  if (!visible.length) return "未设置";
  return `${visible.join("、")}${values.length > limit ? ` 等 ${values.length} 项` : ""}`;
};
const profileKeywordPreview = (profile: ApiMiyunProductProfile) =>
  listPreview(profile.keywords);
const profileMaterialTypePreview = (profile: ApiMiyunProductProfile) =>
  listPreview(profile.material_content_types.map((value) => materialTypeLabel[value] ?? value));
export function miyunJobMaxPages(job: ApiMiyunCrawlJob): number | null {
  const value = (job.query_snapshot as { max_pages?: unknown } | null)?.max_pages;
  return typeof value === "number" && Number.isInteger(value) && value >= 1 && value <= 50
    ? value
    : null;
}
const idempotencyKey = () =>
  globalThis.crypto?.randomUUID?.() ?? `miyun-${Date.now()}`;
const miyunDocumentFile = /\.(pdf|docx|md)$/i;
const miyunProjectMediaFile = /\.(png|jpe?g|webp|mp4)$/i;
const miyunReturnFile = /\.(mp4|zip)$/i;

function crawlContextStorageKey(projectId: string) {
  return `cookies:miyun-crawl-context:v${crawlContextVersion}:${projectId}`;
}

function resolveCrawlContext(projectId: string, jobs: ApiMiyunCrawlJob[]) {
  const validIds = new Set(jobs.map((job) => job.id));
  const urlId = new URLSearchParams(window.location.search).get("crawl_job_id") ?? "";
  if (validIds.has(urlId)) return urlId;
  try {
    const raw = window.localStorage.getItem(crawlContextStorageKey(projectId));
    const stored = raw ? JSON.parse(raw) as { version?: unknown; crawl_job_id?: unknown } : null;
    if (stored?.version === crawlContextVersion && typeof stored.crawl_job_id === "string" && validIds.has(stored.crawl_job_id)) {
      return stored.crawl_job_id;
    }
  } catch {
    // Invalid browser state is ignored; the newest available task becomes the context.
  }
  return jobs[0]?.id ?? "";
}

function persistCrawlContext(projectId: string, crawlJobId: string) {
  try {
    window.localStorage.setItem(crawlContextStorageKey(projectId), JSON.stringify({
      version: crawlContextVersion,
      crawl_job_id: crawlJobId,
    }));
  } catch {
    // URL state remains sufficient when storage is unavailable.
  }
  const url = new URL(window.location.href);
  if (crawlJobId) url.searchParams.set("crawl_job_id", crawlJobId);
  else url.searchParams.delete("crawl_job_id");
  window.history.replaceState(window.history.state, "", `${url.pathname}${url.search}${url.hash}`);
}

function crawlViewHref(view: string, crawlJobId: string) {
  const search = new URLSearchParams({ view, crawl_job_id: crawlJobId });
  return `?${search.toString()}`;
}

type MiyunReturnUpload = {
  handoffId: string;
  file: File;
  expectedVersion: number;
  onProgress: (progress: number) => void;
};

function returnRequestError(status: number) {
  if (status === 403) return new Error("你没有向当前 Project 回传该交接的权限。");
  if (status === 409) return new Error("交接状态已更新，请刷新后重试。");
  return new Error("回传未完成，请检查 MP4 / ZIP 格式、文件大小和素材映射后重试。");
}

async function uploadMiyunHandoffReturn({
  projectId,
  handoffId,
  file,
  expectedVersion,
  onProgress,
}: MiyunReturnUpload & { projectId: string }): Promise<void> {
  const handoffPath = `/api/insights/v1/projects/${encodeURIComponent(projectId)}/miyun/handoffs/${encodeURIComponent(handoffId)}`;
  await new Promise<void>((resolve, reject) => {
    const request = new XMLHttpRequest();
    const endpoint = `${handoffPath}/returns:import`;
    request.open("POST", endpoint);
    request.responseType = "json";
    request.upload.onprogress = (event) => {
      if (event.lengthComputable) onProgress(Math.round((event.loaded / event.total) * 100));
    };
    request.onerror = () => reject(new Error("回传上传失败，请检查网络后重试。"));
    request.onload = () => {
      if (request.status >= 200 && request.status < 300) {
        const result = request.response as { status?: unknown; failed_filenames?: unknown } | null;
        if (result?.status === "partial" || result?.status === "failed") {
          const failedCount = Array.isArray(result.failed_filenames) ? result.failed_filenames.length : 1;
          reject(new Error(`${failedCount} 个文件未完成回传，请检查素材映射或视频文件后重试。`));
          return;
        }
        onProgress(100);
        resolve();
        return;
      }
      reject(returnRequestError(request.status));
    };
    const form = new FormData();
    form.append("file", file);
    form.append("expected_version", String(expectedVersion));
    form.append("idempotency_key", idempotencyKey());
    request.send(form);
  });
}

function miyunAssetLabel(asset: ApiProjectMediaAsset, index: number) {
  const kind = asset.kind === "video" ? "视频" : asset.kind === "image" ? "图片" : "文档";
  return `项目${kind}素材 ${index + 1}`;
}

export function miyunStateCopy(
  status: string,
  scope: "connection" | "job" | "materials" = "job",
) {
  const entries: Record<string, [string, string]> = {
    loading: ["正在读取当前 Project 的米云数据…", "请稍候，或稍后刷新。"],
    empty: ["当前 Project 暂无数据。", "先完成连接与产品分析，再发起采集。"],
    error: ["读取失败，尚未写入任何决定。", "检查服务连接后刷新重试。"],
    forbidden: [
      "你没有访问当前 Project 米云数据的权限。",
      "请联系项目管理员授予访问权限。",
    ],
    partial: [
      "任务仅部分完成，已发现的数据仍可人工查看。",
      "查看失败原因后按需重试任务。",
    ],
    cooling_down: [
      "米云正在冷却，暂不能继续请求。",
      "等待冷却结束后再刷新或重试。",
    ],
    auth_required: [
      "米云授权已失效或需要重新验证。",
      "更新会话后执行“验证连接”。",
    ],
    unverified: ["连接尚未验证。", "保存会话后执行一次只读验证。"],
    ready: [
      "连接已验证，可开始产品分析。",
      "分析产品草稿并由人工确认查询条件。",
    ],
    disabled: ["米云连接未启用。", "提供受权会话并保存后再验证。"],
    failed: [
      "任务或导入失败。",
      scope === "materials"
        ? "核对错误后重试导入。"
        : "查看错误后明确发起重试。",
    ],
  };
  const [title, action] = entries[status] ?? [
    `当前状态：${status}`,
    "刷新以取得服务端最新状态。",
  ];
  return { title, action };
}

export function miyunCrawlErrorCopy(job: {
  last_error_kind?: string;
  last_error_code?: string;
}): string | undefined {
  const kind = job.last_error_kind?.trim();
  const code = job.last_error_code?.trim();
  if (!kind) return undefined;
  const suffix = code ? `（${code}）` : "";
  switch (kind) {
    case "auth_required":
      return `米云授权已失效${suffix}；请先更新会话并重新验证连接。`;
    case "rate_limited":
      return `米云正在限流${suffix}；等待冷却结束后再重试。`;
    case "invalid_request":
    case "graphql_error":
      return `米云未接受冻结的查询条件${suffix}；核对产品关键词、品类和日期窗口后再重试。`;
    default:
      return `安全错误码：${kind}${suffix}`;
  }
}
export function latestMiyunCard(item: MaterialDetail) {
  return item.snapshots[0];
}
export function sortMiyunMaterials(items: MaterialDetail[], field: CardField) {
  return items.slice().sort((a, b) => {
    const ac = latestMiyunCard(a);
    const bc = latestMiyunCard(b);
    const av =
      field === "related_creators" && !ac?.related_creators_known
        ? undefined
        : ac?.[field];
    const bv =
      field === "related_creators" && !bc?.related_creators_known
        ? undefined
        : bc?.[field];
    if (typeof av !== "number")
      return typeof bv !== "number" ? a.id.localeCompare(b.id) : 1;
    return typeof bv !== "number" ? -1 : bv - av;
  });
}
const metric = (value: number | undefined, known = true) =>
  !known
    ? "待确认"
    : typeof value === "number"
      ? value.toLocaleString("zh-CN")
      : "未知";
function errorText(error: unknown) {
  if (error instanceof ApiRequestError && error.code === "VERSION_CONFLICT")
    return "数据已更新，请刷新。";
  if (error instanceof ApiRequestError && error.status === 403)
    return miyunStateCopy("forbidden").title;
  return error instanceof Error ? error.message : "操作失败，请稍后重试。";
}
function miyunHandoffStatusCopy(status: ApiMiyunHandoff["status"]) {
  return {
    exporting: "导出中",
    exported: "已导出（尚未交付）",
    delivered: "已交付（可人工回传）",
    returned: "已回传",
    failed: "导出失败",
  }[status];
}

export function MiyunMaterialsPage({
  state,
  activeView,
}: {
  state: DataState;
  activeView: string;
}) {
  const { currentProject } = useProject();
  const view = viewMap[activeView] ?? "analysis";
  const [connection, setConnection] = useState<ApiMiyunConnection | null>(null);
  const [profiles, setProfiles] = useState<ApiMiyunProductProfile[]>([]);
  const [jobs, setJobs] = useState<ApiMiyunCrawlJob[]>([]);
  const [handoffs, setHandoffs] = useState<ApiMiyunHandoff[]>([]);
  const [materials, setMaterials] = useState<MaterialDetail[]>([]);
  const [materialTotal, setMaterialTotal] = useState(0);
  const [assets, setAssets] = useState<ApiProjectMediaAsset[]>([]);
  const [documents, setDocuments] = useState<ApiKnowledgeDocument[]>([]);
  const [productSource, setProductSource] =
    useState<ApiMiyunProductSource | null>(null);
  const [assetRefs, setAssetRefs] = useState<string[]>([]);
  const [documentIds, setDocumentIds] = useState<string[]>([]);
  const [assetPickerMode, setAssetPickerMode] = useState<"import" | null>(null);
  const [loadState, setLoadState] = useState<
    "loading" | "ready" | "error" | "forbidden"
  >("loading");
  const [busy, setBusy] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [lastSyncedAt, setLastSyncedAt] = useState<Date | null>(null);
  const [notice, setNotice] = useState("");
  const [productId, setProductId] = useState("");
  const [productName, setProductName] = useState("");
  const [categoryName, setCategoryName] = useState("");
  const [draft, setDraft] = useState<ApiMiyunProductProfile | null>(null);
  const [profileId, setProfileId] = useState("");
  const [handoffMaterialIds, setHandoffMaterialIds] = useState<string[]>([]);
  const [notes, setNotes] = useState<Record<string, string>>({});
  const [maxPages, setMaxPages] = useState("50");
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<CardField>("material_score");
  const [materialPage, setMaterialPage] = useState(1);
  const [selectedCrawlJobId, setSelectedCrawlJobId] = useState("");
  const [jobFilterId, setJobFilterId] = useState("");
  const [previewQueue, setPreviewQueue] = useState<string[]>([]);
  const [completedPreviewIds, setCompletedPreviewIds] = useState<string[]>([]);
  const uploadInputRef = useRef<HTMLInputElement>(null);
  const loadRequestIdRef = useRef(0);
  const materialRequestIdRef = useRef(0);
  const backgroundRefreshInFlightRef = useRef(false);
  const lastFocusRefreshRef = useRef(0);
  const sortRef = useRef<CardField>(sort);
  sortRef.current = sort;
  const load = useCallback(async (background = false) => {
    if (!currentProject.id) return;
    const requestId = ++loadRequestIdRef.current;
    if (background) setSyncing(true);
    else {
      setLoadState("loading");
      setNotice("");
    }
    try {
      let nextConnection: ApiMiyunConnection | null = null;
      try {
        nextConnection = await api.getMiyunConnection(currentProject.id);
      } catch (error) {
        if (!(error instanceof ApiRequestError && error.status === 404))
          throw error;
      }
      const [
        profilePage,
        jobPage,
        mediaAssets,
        documentPage,
        source,
        handoffPage,
      ] = await Promise.all([
        api.listMiyunProductProfiles(currentProject.id),
        api.listMiyunCrawlJobs(currentProject.id),
        api.listProjectMediaAssets(currentProject.id),
        api.listKnowledgeDocuments(currentProject.id),
        api.getMiyunProductSource(currentProject.id),
        api.listMiyunHandoffs(currentProject.id),
      ]);
      const crawlJobId = resolveCrawlContext(currentProject.id, jobPage.items);
      const materialResult = crawlJobId
        ? await api.listMiyunMaterials(currentProject.id, {
            crawlJobId,
            limit: materialPageSize,
            sort: sortRef.current,
            handoffEligible: view === "fission",
          })
        : { items: [], total: 0, limit: materialPageSize, offset: 0 };
      const details = await Promise.all(
        materialResult.items.map(async (item) => {
          const detail = await api.getMiyunMaterial(currentProject.id, item.id);
          return {
            ...detail.material,
            snapshots: detail.snapshots.filter((snapshot) => snapshot.crawl_job_id === crawlJobId),
          };
        }),
      );
      if (requestId !== loadRequestIdRef.current) return;
      setConnection(nextConnection);
      setProfiles(profilePage.items);
      setJobs(jobPage.items);
      setSelectedCrawlJobId(crawlJobId);
      persistCrawlContext(currentProject.id, crawlJobId);
      setHandoffs(handoffPage.items);
      setMaterials(details);
      setMaterialTotal(materialResult.total);
      setAssets(mediaAssets);
      setDocuments(documentPage.items);
      setProductSource(source);
      if (source.products.length === 1) {
        setProductId(source.products[0].id);
        setProductName(source.products[0].name);
      }
      setCategoryName(source.category_name);
      setLoadState("ready");
      if (background)
        setNotice((current) => current.includes("自动同步暂未完成") ? "" : current);
      setLastSyncedAt(new Date());
    } catch (error) {
      if (requestId !== loadRequestIdRef.current) return;
      if (background) {
        setNotice(`自动同步暂未完成：${errorText(error)}`);
        return;
      }
      setLoadState(
        error instanceof ApiRequestError && error.status === 403
          ? "forbidden"
          : "error",
      );
      setNotice(errorText(error));
    } finally {
      if (requestId === loadRequestIdRef.current) setSyncing(false);
    }
  }, [currentProject.id, view]);
  useEffect(() => {
    // The workflow reads the persisted connection state only. Verification is
    // an explicit settings action so a transient upstream failure cannot make
    // this page silently downgrade a previously working connection.
    void load();
  }, [load]);
  useEffect(() => {
    if (selectedCrawlJobId && view !== "analysis") {
      persistCrawlContext(currentProject.id, selectedCrawlJobId);
    }
  }, [currentProject.id, selectedCrawlJobId, view]);
  const refreshJobs = useCallback(async () => {
    if (backgroundRefreshInFlightRef.current) return;
    backgroundRefreshInFlightRef.current = true;
    setSyncing(true);
    try {
      const page = await api.listMiyunCrawlJobs(currentProject.id);
      setJobs(page.items);
      setNotice((current) => current.includes("自动同步暂未完成") ? "" : current);
      setLastSyncedAt(new Date());
    } catch (error) {
      setNotice(`任务自动同步暂未完成：${errorText(error)}`);
    } finally {
      backgroundRefreshInFlightRef.current = false;
      setSyncing(false);
    }
  }, [currentProject.id]);
  const refreshMaterials = useCallback(async () => {
    const requestId = ++materialRequestIdRef.current;
    setSyncing(true);
    try {
      const crawlJobId = resolveCrawlContext(currentProject.id, jobs);
      const page = crawlJobId
        ? await api.listMiyunMaterials(currentProject.id, {
            crawlJobId,
            limit: materialPageSize,
            offset: (materialPage - 1) * materialPageSize,
            q: search,
            sort,
            handoffEligible: view === "fission",
          })
        : { items: [], total: 0, limit: materialPageSize, offset: 0 };
      const details = await Promise.all(
        page.items.map(async (item) => {
          const detail = await api.getMiyunMaterial(currentProject.id, item.id);
          return {
            ...detail.material,
            snapshots: detail.snapshots.filter((snapshot) => snapshot.crawl_job_id === crawlJobId),
          };
        }),
      );
      if (requestId !== materialRequestIdRef.current) return;
      setMaterials(details);
      setMaterialTotal(page.total);
      setNotice((current) => current.includes("自动同步暂未完成") ? "" : current);
      setLastSyncedAt(new Date());
    } catch (error) {
      if (requestId === materialRequestIdRef.current) {
        setNotice(`素材自动同步暂未完成：${errorText(error)}`);
      }
    } finally {
      if (requestId === materialRequestIdRef.current) setSyncing(false);
    }
  }, [currentProject.id, jobs, materialPage, search, sort, view]);
  useEffect(() => {
    if (loadState !== "ready" || !["materials", "fission"].includes(view) || !selectedCrawlJobId) return;
    const timer = window.setTimeout(() => void refreshMaterials(), 250);
    return () => window.clearTimeout(timer);
  }, [loadState, refreshMaterials, selectedCrawlJobId, view]);
  const refreshHandoffs = useCallback(async () => {
    if (backgroundRefreshInFlightRef.current) return;
    backgroundRefreshInFlightRef.current = true;
    setSyncing(true);
    try {
      const page = await api.listMiyunHandoffs(currentProject.id);
      setHandoffs(page.items);
      setNotice((current) => current.includes("自动同步暂未完成") ? "" : current);
      setLastSyncedAt(new Date());
    } catch (error) {
      setNotice(`交接自动同步暂未完成：${errorText(error)}`);
    } finally {
      backgroundRefreshInFlightRef.current = false;
      setSyncing(false);
    }
  }, [currentProject.id]);
  const hasActiveJobs = jobs.some((job) => activeJobStatuses.has(job.status));
  const hasActiveImports = materials.some((material) =>
    activeMaterialStatuses.has(material.import_status),
  );
  const hasActiveHandoffs = handoffs.some((handoff) => handoff.status === "exporting");
  useEffect(() => {
    if (loadState !== "ready") return;
    const refresh =
      view === "jobs" && hasActiveJobs
        ? refreshJobs
        : view === "materials" && hasActiveImports
          ? refreshMaterials
          : view === "fission" && hasActiveHandoffs
            ? refreshHandoffs
            : null;
    if (!refresh) return;
    const interval = window.setInterval(() => void refresh(), 5000);
    return () => window.clearInterval(interval);
  }, [
    hasActiveHandoffs,
    hasActiveImports,
    hasActiveJobs,
    loadState,
    refreshHandoffs,
    refreshJobs,
    refreshMaterials,
    view,
  ]);
  useEffect(() => {
    const refreshOnReturn = () => {
      if (document.visibilityState !== "visible") return;
      const now = Date.now();
      if (now - lastFocusRefreshRef.current < 1000) return;
      lastFocusRefreshRef.current = now;
      void load(true);
    };
    window.addEventListener("focus", refreshOnReturn);
    document.addEventListener("visibilitychange", refreshOnReturn);
    return () => {
      window.removeEventListener("focus", refreshOnReturn);
      document.removeEventListener("visibilitychange", refreshOnReturn);
    };
  }, [load]);
  useEffect(() => {
    setNotice("");
    setDraft(null);
    setNotes({});
    setSearch("");
    setAssetRefs([]);
    setDocumentIds([]);
    setHandoffMaterialIds([]);
    setMaxPages("50");
    setMaterialPage(1);
    setPreviewQueue([]);
    setCompletedPreviewIds([]);
    setJobFilterId("");
  }, [currentProject.id, view]);
  const run = async (work: () => Promise<unknown>, message: string) => {
    setBusy(true);
    setNotice("");
    try {
      await work();
      setNotice(message);
      await load(true);
    } catch (error) {
      setNotice(errorText(error));
    } finally {
      setBusy(false);
    }
  };
  const uploadFiles = async (files: FileList | File[]) => {
    const values = Array.from(files);
    if (!values.length) return;
    setBusy(true);
    setNotice("");
    try {
      for (const file of values) {
        if (!miyunDocumentFile.test(file.name) && !miyunProjectMediaFile.test(file.name)) {
          throw new Error("仅支持 PNG、JPG、WEBP、MP4、PDF、DOCX、MD 格式的产品资料。");
        }
        if (miyunDocumentFile.test(file.name)) {
          const document = await api.uploadKnowledgeDocument(
            currentProject.id,
            file,
          );
          setDocuments((current) => [...current, document]);
          setDocumentIds((current) => [...new Set([...current, document.id])]);
        } else {
          const ref = await api.uploadProjectAsset(currentProject.id, file);
          setAssetRefs((current) => [...new Set([...current, ref.asset_id])]);
        }
      }
      await load();
      setNotice("资料已上传并选入本次产品分析。");
    } catch (error) {
      setNotice(errorText(error));
    } finally {
      setBusy(false);
    }
  };
  const selected =
    profiles.find(
      (profile) =>
        profile.id === profileId &&
        (view !== "jobs" || profile.status === "confirmed"),
    ) ??
    (view === "jobs"
      ? profiles.find((profile) => profile.status === "confirmed")
      : profiles[0]);
  const profileById = useMemo(
    () => new Map(profiles.map((profile) => [profile.id, profile])),
    [profiles],
  );
  const displayedProfiles = useMemo(
    () =>
      draft && !profiles.some((profile) => profile.id === draft.id)
        ? [draft, ...profiles]
        : profiles,
    [draft, profiles],
  );
  const parsedMaxPages = Number(maxPages);
  const hasValidMaxPages =
    Number.isInteger(parsedMaxPages) && parsedMaxPages >= 1 && parsedMaxPages <= 50;
  const visible = materials;
  const materialPageCount = Math.max(
    1,
    Math.ceil(materialTotal / materialPageSize),
  );
  const currentMaterialPage = Math.min(materialPage, materialPageCount);
  const paginatedMaterials = visible;
  const previewBatchKey = `${view}:${paginatedMaterials.map((material) => material.id).join("|")}`;
  useEffect(() => {
    const ids = paginatedMaterials.map((material) => material.id);
    setPreviewQueue(ids);
    setCompletedPreviewIds([]);
  }, [previewBatchKey]);
  const remainingPreviewIds = previewQueue.filter((id) => !completedPreviewIds.includes(id));
  const activePreviewIds = remainingPreviewIds.slice(0, previewConcurrency);
  const waitingPreviewCount = Math.max(0, remainingPreviewIds.length - activePreviewIds.length);
  const settlePreview = useCallback((materialId: string) => {
    setCompletedPreviewIds((current) => current.includes(materialId) ? current : [...current, materialId]);
  }, []);
  const requestPreview = useCallback((materialId: string) => {
    setPreviewQueue((current) => current.includes(materialId) ? current : [...current, materialId]);
    setCompletedPreviewIds((current) => current.filter((id) => id !== materialId));
  }, []);
  const hasSelectedSources = assetRefs.length > 0 || documentIds.length > 0;
  const selectedAssets = assets.filter((asset) => assetRefs.includes(asset.id));
  const selectedDocuments = documents.filter((document) =>
    documentIds.includes(document.id),
  );
  const selectedCrawlJob = jobs.find((job) => job.id === selectedCrawlJobId);
  const filteredJobs = jobFilterId
    ? jobs.filter((job) => job.id === jobFilterId)
    : jobs;
  const selectedCrawlProfile = selectedCrawlJob
    ? profileById.get(selectedCrawlJob.product_profile_id)
    : undefined;
  const handoffMaterials = materials;
  const selectedHandoffProfile = selectedCrawlProfile?.status === "confirmed"
    ? selectedCrawlProfile
    : undefined;
  const selectedHandoffs = handoffs.filter((handoff) =>
    handoff.crawl_job_id === selectedCrawlJobId,
  );
  const currentViewCopy = viewCopy[view];
  const toggle = (
    set: React.Dispatch<React.SetStateAction<string[]>>,
    id: string,
  ) =>
    set((current) =>
      current.includes(id)
        ? current.filter((value) => value !== id)
        : [...current, id],
    );
  const cancelJob = async (job: ApiMiyunCrawlJob) => {
    const profile = profileById.get(job.product_profile_id);
    if (!window.confirm(`确认取消“${profile?.product_name ?? "当前产品"}”的采集任务？\n\n取消后不会再请求下一页，已经采集的数据会保留。`))
      return;
    setBusy(true);
    setNotice("");
    try {
      const cancelled = await api.cancelMiyunCrawlJob(
        currentProject.id,
        job.id,
        job.version,
      );
      setJobs((current) =>
        current.map((item) => (item.id === cancelled.id ? cancelled : item)),
      );
      setNotice("任务已取消，不会继续请求下一页；已采集的数据仍会保留。");
      await refreshJobs();
    } catch (error) {
      await refreshJobs();
      setNotice(errorText(error));
    } finally {
      setBusy(false);
    }
  };
  const selectCrawlJob = (crawlJobId: string) => {
    persistCrawlContext(currentProject.id, crawlJobId);
    setSelectedCrawlJobId(crawlJobId);
    setMaterialPage(1);
    setSearch("");
    setNotes({});
    setHandoffMaterialIds([]);
    setPreviewQueue([]);
    setCompletedPreviewIds([]);
    void load(true);
  };
  return (
    <StateBoundary
      state={state}
      onRetry={() => {
        void load();
      }}
    >
      <div
        className="miyun-materials-page"
        aria-busy={busy || loadState === "loading"}
      >
        <header className="core-flow-toolbar miyun-page-toolbar">
          <div className="miyun-page-heading">
            <span className="section-label">{currentViewCopy.eyebrow}</span>
            <h2>{currentViewCopy.title}</h2>
            <p>{currentViewCopy.description}</p>
            <small title={`当前 Project：${currentProject.name}`}>
              当前 Project · {currentProject.name}
            </small>
          </div>
          <div className="miyun-sync-controls" aria-live="polite">
            <span className={syncing ? "syncing" : ""}>
              <i aria-hidden="true" />
              {syncing ? "正在同步" : `已同步至 ${formatSyncTime(lastSyncedAt)}`}
            </span>
            <button
              type="button"
              className="secondary-button"
              disabled={busy || loadState === "loading" || syncing}
              onClick={() => void load(true)}
            >
              <RefreshCw size={15} aria-hidden="true" />
              {syncing ? "同步中" : "立即同步"}
            </button>
          </div>
        </header>
        {loadState === "ready" && view !== "analysis" && view !== "jobs" ? (
          <CrawlContextBar
            jobs={jobs}
            profiles={profiles}
            selectedJobId={selectedCrawlJobId}
            onChange={selectCrawlJob}
          />
        ) : null}
        {view === "fission" ? (
          <>
            <FissionTask
              busy={busy}
              loadState={loadState}
              notice={notice}
              materials={handoffMaterials}
              profile={selectedHandoffProfile}
              crawlJob={selectedCrawlJob}
              handoffs={selectedHandoffs}
              selectedMaterialIds={handoffMaterialIds}
              selectedProfileId={selectedHandoffProfile?.id ?? ""}
              currentPage={currentMaterialPage}
              pageCount={materialPageCount}
              materialTotal={materialTotal}
              projectId={currentProject.id}
              onMaterialChange={(id) => toggle(setHandoffMaterialIds, id)}
              onSelectPage={(ids) =>
                setHandoffMaterialIds((current) => [...new Set([...current, ...ids])])
              }
              onClearSelection={() => setHandoffMaterialIds([])}
              onCreate={() =>
                run(
                  () => api.createMiyunHandoff(currentProject.id, {
                    source_material_ids: handoffMaterialIds,
                    product_profile_id: selectedHandoffProfile!.id,
                    crawl_job_id: selectedCrawlJob!.id,
                  }, idempotencyKey()),
                  "已创建版本化交接；下载成功后才会显示为已导出，交付仍需人工确认。",
                )
              }
              onMarkDelivered={(handoff) =>
                run(
                  () => api.markMiyunHandoffDelivered(currentProject.id, handoff.id, handoff.version),
                  "已人工标记为已交付。",
                )
              }
              onManualReturn={({ handoffId, file, expectedVersion, onProgress }) =>
                uploadMiyunHandoffReturn({
                  projectId: currentProject.id,
                  handoffId,
                  file,
                  expectedVersion,
                  onProgress,
                })
              }
              onRetry={() => void load()}
              onPageChange={setMaterialPage}
            />
          </>
        ) : (
          <>
            {loadState === "loading" ? (
              <div className="panel-empty">
                <b>{miyunStateCopy("loading").title}</b>
                <small>{miyunStateCopy("loading").action}</small>
              </div>
            ) : null}
            {notice ? (
              <div className="inline-notice" role="status">
                {notice}
                {notice === "数据已更新，请刷新。" ? (
                  <button onClick={() => void load()}>刷新</button>
                ) : null}
              </div>
            ) : null}
            {loadState === "forbidden" ? (
              <div className="panel-empty">
                <b>{miyunStateCopy("forbidden").title}</b>
                <small>{miyunStateCopy("forbidden").action}</small>
              </div>
            ) : null}
            {loadState === "error" ? (
              <div className="panel-empty">
                <b>{miyunStateCopy("error").title}</b>
                <small>{miyunStateCopy("error").action}</small>
                <button onClick={() => void load()}>重新读取</button>
              </div>
            ) : null}
            {loadState === "ready" && view === "analysis" ? (
              <>
                <section className="miyun-connection-banner" role="status" aria-live="polite">
                  <div>
                    <span>连接状态</span>
                    <b>{connection ? miyunStateCopy(connection.status, "connection").title : "尚未配置连接"}</b>
                    <small>{connection ? miyunStateCopy(connection.status, "connection").action : "请在系统设置中保存 Cookie 并验证连接。"}</small>
                  </div>
                  <a className="secondary-button" href="/settings">前往系统设置</a>
                </section>
              <section className="miyun-grid">
                <article className="surface-card miyun-analysis-form">
                  <header className="miyun-card-heading">
                    <div>
                      <span>01 · 输入资料</span>
                      <h3>分析产品</h3>
                      <p>选择产品身份并添加可帮助识别产品的图片、视频或文档。</p>
                    </div>
                  </header>
                  <label>
                    已登记产品
                    <select
                      aria-label="选择已登记产品"
                      value={productId}
                      onChange={(e) => {
                        const product = productSource?.products.find(
                          (item) => item.id === e.target.value,
                        );
                        setProductId(e.target.value);
                        setProductName(product?.name ?? "");
                      }}
                    >
                      <option value="">
                        {productSource?.products.length
                          ? "请选择产品"
                          : "暂无已登记产品"}
                      </option>
                      {productSource?.products.map((product) => (
                        <option key={product.id} value={product.id}>
                          {product.name}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    产品名称（可选）
                    <input
                      value={productName}
                      onChange={(e) => setProductName(e.target.value)}
                    />
                  </label>
                  <label>
                    品类名称（可选）
                    <input
                      value={categoryName}
                      onChange={(e) => setCategoryName(e.target.value)}
                    />
                  </label>
                  {!productSource?.products.length ? (
                    <small>
                      当前未登记产品：分析草稿将使用 Project
                      名称作为待确认身份。
                    </small>
                  ) : null}
                  <div
                    className="miyun-upload-zone"
                    onDragOver={(event) => event.preventDefault()}
                    onDrop={(event) => {
                      event.preventDefault();
                      void uploadFiles(event.dataTransfer.files);
                    }}
                  >
                    {!hasSelectedSources ? (
                      <span className="miyun-upload-icon" aria-hidden="true">
                        <Upload size={24} />
                      </span>
                    ) : null}
                    <b>
                      {hasSelectedSources
                        ? `已选 ${assetRefs.length + documentIds.length} 份产品资料`
                        : "把产品资料拖到这里"}
                    </b>
                    {!hasSelectedSources ? (
                      <small>
                        支持 PNG、JPG、WEBP、MP4、PDF、DOCX、MD 等多种格式。资料会按类型安全归入项目素材或知识文档，并自动选入本次分析。
                      </small>
                    ) : null}
                    {hasSelectedSources ? (
                      <div className="miyun-upload-preview-list" role="list">
                        {selectedAssets.map((asset, index) => (
                          <article className="miyun-upload-preview-card" key={asset.id} role="listitem">
                            <span className="miyun-upload-preview-media">
                              {asset.kind === "video" ? (
                              <video muted preload="metadata" src={asset.contentUrl} />
                              ) : asset.kind === "image" ? (
                                <img src={asset.contentUrl} alt={`${miyunAssetLabel(asset, index)}预览`} />
                              ) : (
                                <FileText size={24} aria-hidden="true" />
                              )}
                            </span>
                            <span>
                              <b>{miyunAssetLabel(asset, index)}</b>
                              <small>{asset.mimeType}</small>
                            </span>
                            <button
                              type="button"
                              className="miyun-upload-preview-remove"
                              aria-label={`移除${miyunAssetLabel(asset, index)}`}
                              onClick={() => toggle(setAssetRefs, asset.id)}
                            >
                              <X size={14} />
                            </button>
                          </article>
                        ))}
                        {selectedDocuments.map((document) => (
                          <article className="miyun-upload-preview-card miyun-upload-document-card" key={document.id} role="listitem">
                            <span className="miyun-upload-preview-media">
                              <FileText size={24} aria-hidden="true" />
                            </span>
                            <span>
                              <b>{document.title}</b>
                              <small>{document.mime_type ?? "知识文档"}</small>
                            </span>
                            <button
                              type="button"
                              className="miyun-upload-preview-remove"
                              aria-label={`移除${document.title}`}
                              onClick={() => toggle(setDocumentIds, document.id)}
                            >
                              <X size={14} />
                            </button>
                          </article>
                        ))}
                      </div>
                    ) : null}
                    {hasSelectedSources ? (
                      <small className="miyun-upload-selection-note">
                        项目素材 {assetRefs.length} 项，知识文档 {documentIds.length} 份。继续拖放或从本地上传会自动加入本次分析。
                      </small>
                    ) : null}
                    <div className="miyun-upload-actions">
                      <button
                        type="button"
                        className="primary-button"
                        disabled={busy}
                        onClick={() => uploadInputRef.current?.click()}
                      >
                        {hasSelectedSources ? "继续从本地上传" : "从本地上传"}
                      </button>
                      <button
                        type="button"
                        className="miyun-upload-link"
                        disabled={busy}
                        onClick={() => setAssetPickerMode("import")}
                      >
                        {hasSelectedSources ? "补充已有项目素材" : "从已有项目素材中导入"}
                      </button>
                    </div>
                    <input
                      ref={uploadInputRef}
                      className="miyun-upload-input"
                      aria-label="上传产品分析资料"
                      type="file"
                      multiple
                      accept="image/png,image/jpeg,image/webp,video/mp4,application/pdf,.docx,.md"
                      disabled={busy}
                      onChange={(event) => {
                        if (event.target.files)
                          void uploadFiles(event.target.files);
                        event.currentTarget.value = "";
                      }}
                    />
                  </div>
                  {!hasSelectedSources ? <fieldset>
                    <legend>知识文档（可选）</legend>
                    {!documents.length ? (
                      <small>
                        当前 Project 暂无知识文档；可先导入文档或继续分析。
                      </small>
                    ) : null}
                    {documents.map((document) => (
                      <label key={document.id}>
                        <input
                          type="checkbox"
                          checked={documentIds.includes(document.id)}
                          onChange={() => toggle(setDocumentIds, document.id)}
                        />
                        {document.title}
                      </label>
                    ))}
                  </fieldset> : null}
                  <button
                    className="primary-button"
                    disabled={
                      busy ||
                      !connection ||
                      connection.status !== "ready" ||
                      (!productId &&
                        !productName &&
                        !productSource?.project_name)
                    }
                    onClick={() =>
                      void run(async () => {
                        setDraft(
                          await api.analyzeMiyunProductProfile(
                            currentProject.id,
                            {
                              connection_id: connection!.id,
                              ...(productId ? { product_id: productId } : {}),
                              ...(productName || productSource?.project_name
                                ? {
                                    product_name:
                                      productName ||
                                      productSource!.project_name,
                                  }
                                : {}),
                              ...(categoryName
                                ? { category_name: categoryName }
                                : {}),
                              product_asset_refs: assets
                                .filter((asset) => assetRefs.includes(asset.id))
                                .map((asset) => ({
                                  asset_id: asset.id,
                                  version: asset.version,
                                })),
                              knowledge_document_ids: documentIds,
                            },
                          ),
                        );
                      }, "已生成产品分析草稿；请人工检查后确认。")
                    }
                  >
                    分析产品素材
                  </button>
                </article>
                {assetPickerMode ? (
                  <div
                    className="miyun-asset-picker-backdrop"
                    role="presentation"
                    onMouseDown={(event) => {
                      if (event.target === event.currentTarget)
                        setAssetPickerMode(null);
                    }}
                  >
                    <section
                      className="miyun-asset-picker"
                      role="dialog"
                      aria-modal="true"
                      aria-labelledby="miyun-asset-picker-title"
                      onKeyDown={(event) => {
                        if (event.key === "Escape") setAssetPickerMode(null);
                      }}
                    >
                      <header>
                        <div>
                          <span className="section-label">PROJECT ASSET LIBRARY</span>
                          <h3 id="miyun-asset-picker-title">从已有项目素材中导入</h3>
                          <p>预览并补充 Project 素材，不会影响上传区内已选资料。</p>
                        </div>
                        <button
                          type="button"
                          aria-label="关闭项目素材选择"
                          onClick={() => setAssetPickerMode(null)}
                        >
                          <X size={16} />
                        </button>
                      </header>
                      {!assets.length ? (
                        <div className="panel-empty">
                          当前 Project 暂无可预览素材。可直接从本地上传。
                        </div>
                      ) : (
                        <div className="miyun-asset-picker-grid">
                          {assets.map((asset, index) => {
                            const selectedAsset = assetRefs.includes(asset.id);
                            return (
                              <button
                                type="button"
                                key={asset.id}
                                className={`miyun-asset-choice${selectedAsset ? " selected" : ""}`}
                                aria-pressed={selectedAsset}
                                onClick={() => toggle(setAssetRefs, asset.id)}
                              >
                                <span className="miyun-asset-choice-preview">
                                  {asset.kind === "video" ? (
                                    <video muted preload="metadata" src={asset.contentUrl} />
                                  ) : asset.kind === "image" ? (
                                    <img src={asset.contentUrl} alt="" />
                                  ) : (
                                    <span>文档素材</span>
                                  )}
                                </span>
                                <span>
                                  <b>{miyunAssetLabel(asset, index)}</b>
                                  <small>
                                    {asset.mimeType} · v{asset.version}
                                  </small>
                                </span>
                                {selectedAsset ? <Check size={16} aria-label="已选" /> : null}
                              </button>
                            );
                          })}
                        </div>
                      )}
                      <footer>
                        <small>本次分析已选择 {assetRefs.length} 项项目素材</small>
                        <button
                          type="button"
                          className="primary-button"
                          onClick={() => setAssetPickerMode(null)}
                        >
                          完成导入
                        </button>
                      </footer>
                    </section>
                  </div>
                ) : null}
                <article className="surface-card miyun-profile-panel">
                  <header className="miyun-card-heading">
                    <div>
                      <span>02 · 人工确认</span>
                      <h3>产品分析草稿</h3>
                      <p>选择一版草稿，核对关键词和素材类型后再用于采集。</p>
                    </div>
                  </header>
                  {!profiles.length && !draft ? (
                    <div className="panel-empty">
                      暂无草稿。请先验证连接并分析产品。
                    </div>
                  ) : (
                    <div className="miyun-profile-list">
                      {displayedProfiles.map((profile) => (
                        <button
                          type="button"
                          className="miyun-profile-card"
                          aria-pressed={draft?.id === profile.id}
                          key={profile.id}
                          onClick={() => {
                            setProfileId(profile.id);
                            setDraft({
                              ...profile,
                              keywords: [...profile.keywords],
                              material_content_types: [
                                ...profile.material_content_types,
                              ],
                            });
                          }}
                        >
                          <span>
                            <b>{profile.product_name}</b>
                            <small>关键词 · {profileKeywordPreview(profile)}</small>
                            <small>素材类型 · {profileMaterialTypePreview(profile)}</small>
                            <small>查询窗口 · {miyunCalendarDate(profile.window_start)} 至 {miyunCalendarDate(profile.window_end)}</small>
                          </span>
                          <span className={`miyun-status-badge ${profile.status}`}>
                            {profile.status === "confirmed" ? "已确认" : profile.status === "draft" ? "待确认" : "已被替代"}
                            <small>v{profile.version}</small>
                          </span>
                        </button>
                      ))}
                    </div>
                  )}
                </article>
                {draft ? (
                  <ProfileEditor
                    draft={draft}
                    busy={busy}
                    onChange={setDraft}
                    onConfirm={() =>
                      void run(
                        async () => {
                          const confirmed = await api.confirmMiyunProductProfile(
                            currentProject.id,
                            draft.id,
                            draft.version,
                            profileQuery(draft),
                          );
                          setDraft(confirmed);
                          setProfileId(confirmed.id);
                        },
                        "查询条件已确认。下一步可创建采集任务。",
                      )
                    }
                  />
                ) : null}
              </section>
              </>
            ) : null}
            {loadState === "ready" && view === "jobs" ? (
              <section className="miyun-jobs-layout">
                <article className="surface-card miyun-job-create-card">
                  <header className="miyun-card-heading">
                    <div>
                      <span>新任务</span>
                      <h3>创建采集任务</h3>
                      <p>采集使用已人工确认的产品画像，创建后会在右侧自动更新进度。</p>
                    </div>
                  </header>
                  <label>
                    产品画像
                    <select
                      aria-label="产品分析"
                      name="miyun-product-profile"
                      value={selected?.id ?? ""}
                      onChange={(e) => setProfileId(e.target.value)}
                    >
                      {profiles
                        .filter((profile) => profile.status === "confirmed")
                        .map((profile) => (
                          <option key={profile.id} value={profile.id}>
                            {profile.product_name} · {profileKeywordPreview(profile)} · {profileMaterialTypePreview(profile)}
                          </option>
                        ))}
                    </select>
                  </label>
                  {selected ? (
                    <dl className="miyun-selected-profile-preview">
                      <div><dt>关键词</dt><dd>{profileKeywordPreview(selected)}</dd></div>
                      <div><dt>素材类型</dt><dd>{profileMaterialTypePreview(selected)}</dd></div>
                      <div><dt>查询窗口</dt><dd>{miyunCalendarDate(selected.window_start)} 至 {miyunCalendarDate(selected.window_end)}</dd></div>
                    </dl>
                  ) : null}
                  <label>
                    最大抓取页数
                    <input
                      type="number"
                      inputMode="numeric"
                      name="miyun-max-pages"
                      autoComplete="off"
                      min="1"
                      max="50"
                      step="1"
                      value={maxPages}
                      onChange={(event) => setMaxPages(event.target.value)}
                      aria-describedby="miyun-max-pages-help"
                    />
                    <small id="miyun-max-pages-help">最多 50 页；达到上限或米云返回最后一页时自动结束。</small>
                  </label>
                  {!profiles.some((profile) => profile.status === "confirmed") ? (
                    <div className="miyun-context-note">
                      尚无已确认的产品画像。请先在“产品分析”中确认查询条件。
                    </div>
                  ) : null}
                  {connection?.status !== "ready" ? (
                    <div className="miyun-context-note error" role="status">
                      <b>当前连接不可用于采集</b>
                      <span>{connection ? miyunStateCopy(connection.status, "connection").action : "尚未配置米云连接。"}</span>
                      <a href="/settings">前往系统设置检查连接</a>
                    </div>
                  ) : null}
                  {!hasValidMaxPages ? (
                    <div className="miyun-context-note error" role="alert">最大抓取页数必须是 1–50 的整数。</div>
                  ) : null}
                  <button
                    type="button"
                    className="primary-button"
                    disabled={
                      busy ||
                      !selected ||
                      selected.status !== "confirmed" ||
                      connection?.status !== "ready" ||
                      !hasValidMaxPages
                    }
                    onClick={() =>
                      void run(
                        async () => {
                          const created = await api.createMiyunCrawlJob(
                            currentProject.id,
                            {
                              product_profile_id: selected!.id,
                              operation: "product",
                              max_pages: parsedMaxPages,
                            },
                            idempotencyKey(),
                          );
                          persistCrawlContext(currentProject.id, created.id);
                          setSelectedCrawlJobId(created.id);
                        },
                        `采集任务已创建，最多抓取 ${parsedMaxPages} 页。`,
                      )
                    }
                  >
                    创建产品采集任务
                  </button>
                </article>
                <article className="surface-card miyun-job-progress-card">
                  <header className="miyun-card-heading miyun-card-heading-inline">
                    <div>
                      <span>任务队列</span>
                      <h3>采集进度</h3>
                      <p>{hasActiveJobs ? "运行中的任务每 5 秒自动更新。" : "新任务开始运行后将自动更新。"}</p>
                    </div>
                    <div className="miyun-job-filter-actions">
                      <label>
                        采集任务切换
                        <select
                          aria-label="筛选采集任务"
                          value={jobFilterId}
                          onChange={(event) => setJobFilterId(event.target.value)}
                        >
                          <option value="">全部采集任务</option>
                          {jobs.map((job) => {
                            const jobProfile = profileById.get(job.product_profile_id);
                            return (
                              <option key={job.id} value={job.id}>
                                {jobProfile?.product_name ?? "未知产品"} · {formatServerTime(job.created_at)} · {job.id.slice(-8)}
                              </option>
                            );
                          })}
                        </select>
                      </label>
                      <span className="miyun-record-count">
                        {jobFilterId ? `${filteredJobs.length} / ${jobs.length}` : jobs.length} 个任务
                      </span>
                    </div>
                  </header>
                  {!jobs.length ? (
                    <div className="panel-empty">
                      暂无采集任务。请先人工确认产品查询条件，再明确创建采集任务。
                    </div>
                  ) : (
                    <div className="miyun-job-list">
                      {filteredJobs.map((job) => {
                        const jobProfile = profileById.get(job.product_profile_id);
                        const jobMaxPages = miyunJobMaxPages(job);
                        return <article className="miyun-job-row" key={job.id}>
                          <header>
                            <div>
                              <b>{jobProfile?.product_name ?? (job.operation === "product" ? "产品素材采集" : "CID 素材采集")}</b>
                              {jobProfile ? <small className="miyun-job-profile-summary">关键词 · {profileKeywordPreview(jobProfile)} ｜ 素材类型 · {profileMaterialTypePreview(jobProfile)}</small> : null}
                              <small title={job.id}>{job.id}</small>
                            </div>
                            <span className={`miyun-status-badge ${job.status}`}>
                              {jobStatusLabel[job.status]}
                            </span>
                          </header>
                          <dl className="miyun-metric-grid compact job">
                            <div><dt>完成 / 上限页数</dt><dd>{job.completed_pages} / {jobMaxPages ?? "旧任务未限制"}</dd></div>
                            <div><dt>发现</dt><dd>{job.discovered_count}</dd></div>
                            <div><dt>去重</dt><dd>{job.deduplicated_count}</dd></div>
                            <div><dt>下载</dt><dd>{job.downloaded_count}</dd></div>
                            <div><dt>失败</dt><dd>{job.failed_count}</dd></div>
                          </dl>
                          <div className="miyun-job-guidance">
                            <p>{miyunStateCopy(job.status).action}</p>
                            {miyunCrawlErrorCopy(job) ? (
                              <p className="error">{miyunCrawlErrorCopy(job)}</p>
                            ) : null}
                            {job.status === "cooling_down" && job.cooldown_until ? (
                              <p>预计恢复：{formatServerTime(job.cooldown_until)}</p>
                            ) : null}
                            <small>更新于 {formatServerTime(job.updated_at)}</small>
                          </div>
                          <div className="miyun-card-actions">
                            <a
                              className="secondary-button"
                              href={crawlViewHref("素材候选", job.id)}
                              onClick={() => persistCrawlContext(currentProject.id, job.id)}
                            >
                              查看该批次素材
                            </a>
                            <a
                              className="secondary-button"
                              href={crawlViewHref("裂变任务", job.id)}
                              onClick={() => persistCrawlContext(currentProject.id, job.id)}
                            >
                              进入该批次裂变
                            </a>
                            {["queued", "running", "cooling_down"].includes(job.status) ? (
                              <button
                                type="button"
                                className="secondary-button"
                                disabled={busy}
                                onClick={() => void cancelJob(job)}
                              >
                                取消任务
                              </button>
                            ) : null}
                            {["failed", "cancelled", "partial"].includes(job.status) ? (
                              <button
                                type="button"
                                className="primary-button"
                                disabled={busy}
                                onClick={() =>
                                  void run(
                                    () => api.retryMiyunCrawlJob(currentProject.id, job.id, idempotencyKey()),
                                    "已创建重试任务。",
                                  )
                                }
                              >
                                <RotateCcw size={14} aria-hidden="true" />
                                创建重试任务
                              </button>
                            ) : null}
                          </div>
                        </article>;
                      })}
                    </div>
                  )}
                </article>
              </section>
            ) : null}
            {loadState === "ready" && view === "materials" ? (
              <section className="miyun-material-list">
                <div className="prelaunch-filterbar">
                  <div className="search-field">
                    <Search size={15} />
                    <input
                      aria-label="搜索素材候选"
                      name="miyun-material-search"
                      placeholder="搜索素材标题或米云 ID…"
                      value={search}
                      onChange={(e) => {
                        setSearch(e.target.value);
                        setMaterialPage(1);
                      }}
                    />
                  </div>
                  <label className="miyun-sort-control">
                    排序方式
                    <select
                      aria-label="按数据卡排序"
                      name="miyun-material-sort"
                      value={sort}
                      onChange={(e) => {
                        setSort(e.target.value as CardField);
                        setMaterialPage(1);
                      }}
                    >
                      <option value="delivery_days">投放天数</option>
                      <option value="cumulative_impressions">累计曝光</option>
                      <option value="related_ads">关联广告</option>
                      <option value="related_creators">关联达人</option>
                      <option value="material_score">素材分</option>
                    </select>
                  </label>
                  <small>
                    共 {materialTotal} 条 · 采用所选采集任务的数据卡，未知值始终排在最后
                  </small>
                  <small className="miyun-preview-batch-status" aria-live="polite">
                    预览按每批 {previewConcurrency} 条自动加载 · 当前 {activePreviewIds.length} 条进行中 · {waitingPreviewCount} 条等待中
                  </small>
                </div>
                {!materialTotal ? (
                  <div className="panel-empty">
                    <b>{miyunStateCopy("empty", "materials").title}</b>
                    <small>请先完成采集；也可清除搜索条件后重新查看。</small>
                  </div>
                ) : null}
                {materialTotal ? (
                  <nav className="miyun-pagination" aria-label="素材候选分页">
                    <span aria-live="polite">
                      第 {currentMaterialPage} / {materialPageCount} 页 · 共 {materialTotal} 条 · 每页 {materialPageSize} 条
                    </span>
                    <div>
                      <button
                        type="button"
                        className="secondary-button"
                        disabled={currentMaterialPage === 1}
                        onClick={() => {
                          setMaterialPage(Math.max(1, currentMaterialPage - 1));
                        }}
                      >
                        上一页
                      </button>
                      <button
                        type="button"
                        className="secondary-button"
                        disabled={currentMaterialPage === materialPageCount}
                        onClick={() => {
                          setMaterialPage(Math.min(materialPageCount, currentMaterialPage + 1));
                        }}
                      >
                        下一页
                      </button>
                    </div>
                  </nav>
                ) : null}
                <div className="miyun-material-grid">
                {paginatedMaterials.map((material) => (
                  <MaterialCard
                    key={material.id}
                    material={material}
                    note={notes[material.id] ?? ""}
                    busy={busy}
                    previewActive={activePreviewIds.includes(material.id)}
                    onPreviewRequest={() => requestPreview(material.id)}
                    onPreviewSettled={() => settlePreview(material.id)}
                    onNote={(value) =>
                      setNotes((current) => ({ ...current, [material.id]: value }))
                    }
                    onConfirm={() =>
                      void run(
                        () =>
                          api.confirmMiyunMaterial(
                            currentProject.id,
                            material.id,
                            material.version,
                            notes[material.id] ?? "",
                          ),
                        "素材已人工确认，并由服务端排队导入。",
                      )
                    }
                    onReject={() =>
                      void run(
                        () =>
                          api.rejectMiyunMaterial(
                            currentProject.id,
                            material.id,
                            material.version,
                            notes[material.id] ?? "",
                          ),
                        "素材已人工拒绝。",
                      )
                    }
                    onRetry={() =>
                      void run(
                        () =>
                          api.retryMiyunMaterialImport(
                            currentProject.id,
                            material.id,
                            material.version,
                          ),
                        "失败导入已请求重试。",
                      )
                    }
                    preview={
                      material.import_method === "crawler"
                        ? api.getMiyunMaterialPreviewUrl(
                            currentProject.id,
                            material.id,
                          )
                        : undefined
                    }
                  />
                ))}
                </div>
              </section>
            ) : null}
          </>
        )}
      </div>
    </StateBoundary>
  );
}
function CrawlContextBar({
  jobs,
  profiles,
  selectedJobId,
  onChange,
}: {
  jobs: ApiMiyunCrawlJob[];
  profiles: ApiMiyunProductProfile[];
  selectedJobId: string;
  onChange: (jobId: string) => void;
}) {
  const profileMap = new Map(profiles.map((profile) => [profile.id, profile]));
  const selectedJob = jobs.find((job) => job.id === selectedJobId);
  const selectedProfile = selectedJob ? profileMap.get(selectedJob.product_profile_id) : undefined;
  return (
    <section className="miyun-crawl-context" aria-label="当前采集任务">
      <div>
        <span className="section-label">当前采集任务</span>
        <b>{selectedProfile?.product_name ?? "请选择采集任务"}</b>
        {selectedJob ? (
          <small>
            {formatServerTime(selectedJob.created_at)} · {jobStatusLabel[selectedJob.status]} · 发现 {selectedJob.discovered_count} 条
            {selectedProfile ? ` · ${profileKeywordPreview(selectedProfile)}` : ""}
          </small>
        ) : (
          <small>从采集任务开始，素材审核与裂变交接都绑定到同一个结果批次。</small>
        )}
      </div>
      <label>
        采集任务切换
        <select
          aria-label="切换当前采集任务"
          value={selectedJobId}
          onChange={(event) => onChange(event.target.value)}
        >
          {!jobs.length ? <option value="">暂无采集任务</option> : null}
          {jobs.map((job) => {
            const profile = profileMap.get(job.product_profile_id);
            return (
              <option key={job.id} value={job.id}>
                {profile?.product_name ?? "未知产品"} · {formatServerTime(job.created_at)} · {jobStatusLabel[job.status]} · {job.id.slice(-8)}
              </option>
            );
          })}
        </select>
      </label>
    </section>
  );
}
function FissionTask({
  busy,
  loadState,
  notice,
  materials,
  profile,
  crawlJob,
  handoffs,
  selectedMaterialIds,
  selectedProfileId,
  currentPage,
  pageCount,
  materialTotal,
  projectId,
  onMaterialChange,
  onSelectPage,
  onClearSelection,
  onCreate,
  onMarkDelivered,
  onManualReturn,
  onRetry,
  onPageChange,
}: {
  busy: boolean;
  loadState: "loading" | "ready" | "error" | "forbidden";
  notice: string;
  materials: MaterialDetail[];
  profile?: ApiMiyunProductProfile;
  crawlJob?: ApiMiyunCrawlJob;
  handoffs: ApiMiyunHandoff[];
  selectedMaterialIds: string[];
  selectedProfileId: string;
  currentPage: number;
  pageCount: number;
  materialTotal: number;
  projectId: string;
  onMaterialChange: (id: string) => void;
  onSelectPage: (ids: string[]) => void;
  onClearSelection: () => void;
  onCreate: () => Promise<void>;
  onMarkDelivered: (handoff: ApiMiyunHandoff) => Promise<void>;
  onManualReturn: (upload: MiyunReturnUpload) => Promise<void>;
  onRetry: () => void;
  onPageChange: (page: number) => void;
}) {
  // Older persisted profiles can contain JSON null for optional source lists.
  // Normalize at the UI boundary so a valid confirmed profile never crashes
  // the handoff selector while its frozen file counts are rendered.
  const normalizedProfile = profile && {
    ...profile,
    product_asset_refs: profile.product_asset_refs ?? [],
    knowledge_document_ids: profile.knowledge_document_ids ?? [],
  };
  const pageMaterialIds = materials.map((material) => material.id);
  const isPageSelected =
    pageMaterialIds.length > 0 &&
    pageMaterialIds.every((id) => selectedMaterialIds.includes(id));
  return (
    <section className="miyun-grid" aria-label="裂变任务">
      <article className="surface-card miyun-handoff-create-card">
        <header className="miyun-card-heading">
          <div>
            <span>新交接</span>
            <h3>创建裂变交接</h3>
            <p>选择已确认素材与产品画像，冻结为可追溯的输入版本。</p>
          </div>
        </header>
        <div className="miyun-context-note">交接不会调用外部 AI，也不会自动发布或标记为已交付。</div>
        {loadState !== "ready" ? (
          <div className="panel-empty">
            {notice || miyunStateCopy(loadState).title}
            {loadState !== "loading" ? <button type="button" onClick={onRetry}>刷新</button> : null}
          </div>
        ) : (
          <>
            <fieldset className="miyun-handoff-material-picker">
              <legend>已确认且已入库的爆款素材</legend>
              <div className="miyun-handoff-selection-toolbar">
                <label>
                  <input
                    type="checkbox"
                    checked={isPageSelected}
                    disabled={!pageMaterialIds.length}
                    onChange={() => {
                      if (isPageSelected) {
                        pageMaterialIds.forEach((id) => {
                          if (selectedMaterialIds.includes(id)) onMaterialChange(id);
                        });
                      } else {
                        onSelectPage(pageMaterialIds);
                      }
                    }}
                  />
                  全选本页
                </label>
                <button
                  type="button"
                  className="text-button"
                  disabled={!selectedMaterialIds.length}
                  onClick={onClearSelection}
                >
                  清空已选（{selectedMaterialIds.length}）
                </button>
              </div>
              <div className="miyun-handoff-material-options" role="group" aria-label="handoff-source-materials">
                {materials.map((material) => <label key={material.id}><input type="checkbox" checked={selectedMaterialIds.includes(material.id)} onChange={() => onMaterialChange(material.id)} /><span><b>{material.title ?? material.miyun_material_id}</b><small>{importStatusLabel[material.import_status]} · v{material.version}</small></span></label>)}
              </div>
            </fieldset>
            {materialTotal ? (
              <nav className="miyun-pagination" aria-label="裂变素材分页">
                <span>第 {currentPage} / {pageCount} 页 · 当前已选 {selectedMaterialIds.length} 条 · 批次共 {materialTotal} 条候选</span>
                <div>
                  <button type="button" className="secondary-button" disabled={currentPage === 1} onClick={() => onPageChange(Math.max(1, currentPage - 1))}>上一页</button>
                  <button type="button" className="secondary-button" disabled={currentPage === pageCount} onClick={() => onPageChange(Math.min(pageCount, currentPage + 1))}>下一页</button>
                </div>
              </nav>
            ) : null}
            {normalizedProfile && crawlJob ? (
              <div className="miyun-handoff-profile-files" role="note">
                <b>绑定批次与产品画像</b>
                <small>
                  {normalizedProfile.product_name} · 任务 {crawlJob.id}。将冻结媒体 {normalizedProfile.product_asset_refs.length} 项、文档 {normalizedProfile.knowledge_document_ids.length} 项，以及该批次的数据卡。
                </small>
              </div>
            ) : null}
            {!materials.length || !normalizedProfile ? <small>{!materials.length ? "该采集任务暂无已确认且已入库/去重的爆款素材。" : "当前采集任务没有可用的已确认产品画像。"}</small> : null}
            <button className="primary-button" disabled={busy || !selectedMaterialIds.length || !selectedProfileId || !crawlJob} onClick={() => void onCreate()}>创建该批次交接</button>
          </>
        )}
      </article>
      <article className="surface-card miyun-handoff-history-card">
        <header className="miyun-card-heading miyun-card-heading-inline">
          <div>
            <span>历史记录</span>
            <h3>交接历史</h3>
            <p>源素材与项目资料分别生成扁平 ZIP；两个压缩包均需上传至 AI 系统。</p>
          </div>
          <span className="miyun-record-count">{handoffs.length} 个交接</span>
        </header>
        {!handoffs.length ? <div className="panel-empty">暂无交接记录。</div> : handoffs.map((handoff) => (
          <div className="miyun-handoff-history-row" key={handoff.id}>
            <header>
              <span><b>裂变交接</b><small title={handoff.id}>{handoff.id}</small></span>
              <span className={`miyun-status-badge ${handoff.status}`}>{miyunHandoffStatusCopy(handoff.status)}</span>
            </header>
            <dl className="miyun-handoff-metadata">
              <div><dt>素材</dt><dd>{handoff.source_material_ids.length} 条</dd></div>
              <div><dt>版本</dt><dd>v{handoff.version}</dd></div>
              <div><dt>更新</dt><dd>{formatServerTime(handoff.updated_at)}</dd></div>
            </dl>
            <div className="miyun-card-actions">
              {handoff.status === "exporting" || handoff.status === "exported" || handoff.status === "delivered" || handoff.status === "returned" ? (
                <>
                  <a className="secondary-button" href={api.getMiyunHandoffExportUrl(projectId, handoff.id, "sources")} download onClick={() => window.setTimeout(() => void onRetry(), 1200)}>下载裂变源素材 ZIP</a>
                  <a className="secondary-button" href={api.getMiyunHandoffExportUrl(projectId, handoff.id, "project")} download onClick={() => window.setTimeout(() => void onRetry(), 1200)}>下载项目资料 ZIP</a>
                </>
              ) : null}
              {handoff.status === "exported" ? <button type="button" className="primary-button" disabled={busy} onClick={() => void onMarkDelivered(handoff)}>确认已交付</button> : null}
            </div>
            {handoff.status === "exported" || handoff.status === "delivered" || handoff.status === "returned" ? (
              <ManualHandoffReturnUpload
                handoff={handoff}
                busy={busy}
                onUpload={onManualReturn}
                onComplete={onRetry}
              />
            ) : null}
            {handoff.status === "returned" ? (
              <div role="status" className="miyun-handoff-returned">
                <b>已回传</b>
                <small>已有裂变素材回传；仍可继续为当前采集任务补充 MP4 或 ZIP。</small>
              </div>
            ) : null}
            {handoff.returns?.length ? <small>回传历史：{handoff.returns.map((item) => `${item.status}${item.filename ? ` · ${item.filename}` : ""} · ${item.source_material_id ? `源素材 ${item.source_material_id}` : `采集任务 ${item.crawl_job_id ?? handoff.crawl_job_id ?? "当前批次"}`}${item.sha256 ? ` · SHA-256 ${item.sha256}` : ""}`).join("；")}</small> : null}
            <details className="miyun-lineage-details">
              <summary>查看版本与输入血缘</summary>
              <small>manifest {handoff.manifest_version} · 参数 {handoff.parameter_version}</small>
              <small>血缘：本 Project · 素材 {handoff.source_material_ids.join("、")}</small>
              <small>profile {handoff.product_profile_id}</small>
              <small>输入哈希 <code>{handoff.input_hash}</code></small>
            </details>
            {handoff.status === "exporting" ? <small>等待导出：分别下载两个 ZIP；任一压缩包成功生成后可人工确认交付，请在交付前确认两个文件都已下载。</small> : null}
            {handoff.status === "failed" ? <small>导出失败；请重新下载，失败不会标记为已导出或已交付。</small> : null}
          </div>
        ))}
      </article>
    </section>
  );
}

function ManualHandoffReturnUpload({
  handoff,
  busy,
  onUpload,
  onComplete,
}: {
  handoff: ApiMiyunHandoff;
  busy: boolean;
  onUpload: (upload: MiyunReturnUpload) => Promise<void>;
  onComplete: () => void;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [progress, setProgress] = useState<number | null>(null);
  const [error, setError] = useState("");
  const upload = async () => {
    if (!file) {
      setError("请选择要人工回传的 MP4 或 ZIP 文件。");
      return;
    }
    setError("");
    setProgress(0);
    try {
      await onUpload({
        handoffId: handoff.id,
        file,
        expectedVersion: handoff.version,
        onProgress: setProgress,
      });
      setFile(null);
      onComplete();
    } catch (cause) {
      // The transport helper deliberately produces safe messages only; never
      // render server response bodies, which can contain storage paths.
      setError(cause instanceof Error ? cause.message : "回传未完成，请重试。");
    } finally {
      setProgress(null);
    }
  };
  return (
    <div className="miyun-handoff-return-upload" aria-label={`人工回传 ${handoff.id}`}>
      <b>人工回传 MP4 / ZIP</b>
      <small>无元信息时默认关联当前采集任务；文件名使用“源素材 ID__名称.mp4”，或在扁平 ZIP 中提供 manifest.xlsx，可精确关联源素材。ZIP 最多包含 20 个 MP4。</small>
      <input
        type="file"
        accept="video/mp4,application/zip,.mp4,.zip"
        aria-label={`选择 ${handoff.id} 的回传 MP4 或 ZIP`}
        disabled={busy || progress !== null}
        onChange={(event) => {
          const selected = event.target.files?.[0] ?? null;
          setError(selected && !miyunReturnFile.test(selected.name) ? "仅支持 MP4 或扁平 ZIP 文件回传。" : "");
          setFile(selected && miyunReturnFile.test(selected.name) ? selected : null);
        }}
      />
      {file ? <small>已选择：{file.name}</small> : null}
      {progress !== null ? <progress max="100" value={progress} aria-label="回传上传进度">{progress}%</progress> : null}
      <button type="button" className="primary-button" disabled={busy || progress !== null || !file} onClick={() => void upload()}>
        {progress !== null ? `正在回传 ${progress}%` : "人工上传回传"}
      </button>
      {error ? <div className="inline-notice" role="alert">{error}<button type="button" className="secondary-button" disabled={busy || progress !== null || !file} onClick={() => void upload()}>重试回传</button></div> : null}
    </div>
  );
}

export function profileQuery(profile: ApiMiyunProductProfile): ApiMiyunProfileQuery {
  return {
    product_name: profile.product_name,
    category_id: profile.category_id,
    category_name: profile.category_name,
    keywords: profile.keywords,
    material_content_types: profile.material_content_types,
    // Go serializes persisted date-only values as RFC3339 timestamps, while
    // the confirmation endpoint intentionally accepts calendar dates only.
    window_start: miyunCalendarDate(profile.window_start),
    window_end: miyunCalendarDate(profile.window_end),
  };
}
function miyunCalendarDate(value: string): string {
  const date = /^\d{4}-\d{2}-\d{2}/.exec(value.trim());
  return date?.[0] ?? value.trim();
}
function ProfileEditor({
  draft,
  busy,
  onChange,
  onConfirm,
}: {
  draft: ApiMiyunProductProfile;
  busy: boolean;
  onChange: (value: ApiMiyunProductProfile) => void;
  onConfirm: () => void;
}) {
  const confirmed = draft.status === "confirmed";
  return (
    <article className="surface-card miyun-profile-editor">
      <header className="miyun-card-heading">
        <div>
          <span>03 · 查询条件</span>
          <h3>人工编辑并确认查询</h3>
          <p>这些条件会被冻结在产品画像中，后续采集不会静默改写。</p>
        </div>
      </header>
      <label>
        关键词（逗号分隔）
        <input
          name="miyun-profile-keywords"
          autoComplete="off"
          disabled={confirmed}
          value={draft.keywords.join(", ")}
          onChange={(e) =>
            onChange({
              ...draft,
              keywords: e.target.value
                .split(",")
                .map((x) => x.trim())
                .filter(Boolean),
            })
          }
        />
      </label>
      <label>
        素材类型（逗号分隔）
        <input
          name="miyun-profile-material-types"
          autoComplete="off"
          disabled={confirmed}
          value={draft.material_content_types.join(", ")}
          onChange={(e) =>
            onChange({
              ...draft,
              material_content_types: e.target.value
                .split(",")
                .map((x) => x.trim())
                .filter(Boolean),
            })
          }
        />
      </label>
      {confirmed ? (
        <section className="miyun-next-step" aria-labelledby="miyun-next-step-title">
          <span aria-hidden="true"><Check size={18} /></span>
          <div>
            <b id="miyun-next-step-title">查询条件已确认</b>
            <p>画像已经冻结，可以用这组关键词和素材类型创建采集任务。</p>
          </div>
          <a className="primary-button" href={`?view=${encodeURIComponent("采集任务")}`}>
            下一步：创建采集任务
          </a>
        </section>
      ) : (
        <button
          type="button"
          className="primary-button"
          disabled={busy || !draft.keywords.length}
          onClick={onConfirm}
        >
          确认查询条件
        </button>
      )}
    </article>
  );
}
function MaterialCard({
  material,
  note,
  busy,
  previewActive,
  onPreviewRequest,
  onPreviewSettled,
  onNote,
  onConfirm,
  onReject,
  onRetry,
  preview,
}: {
  material: MaterialDetail;
  note: string;
  busy: boolean;
  previewActive: boolean;
  onPreviewRequest: () => void;
  onPreviewSettled: () => void;
  onNote: (value: string) => void;
  onConfirm: () => void;
  onReject: () => void;
  onRetry: () => void;
  preview?: string;
}) {
  const card = latestMiyunCard(material);
  const [previewAttempt, setPreviewAttempt] = useState(1);
  const [previewReady, setPreviewReady] = useState(false);
  const [previewFailed, setPreviewFailed] = useState(false);
  const requestPreview = () => {
    onPreviewRequest();
    setPreviewReady(false);
    setPreviewFailed(false);
    setPreviewAttempt((attempt) => attempt + 1);
  };
  return (
    <article className="surface-card miyun-material-card">
      <header className="miyun-material-card-heading">
        <div>
          <span className="section-label">素材候选</span>
          <h3>{material.title ?? material.miyun_material_id}</h3>
          <small title={material.miyun_material_id}>米云 ID · {material.miyun_material_id}</small>
        </div>
        <div className="miyun-material-statuses">
          <span className={`miyun-status-badge ${material.selection_status}`}>
            {materialStatusLabel[material.selection_status]}
          </span>
          <span className={`miyun-status-badge ${material.import_status}`}>
            {importStatusLabel[material.import_status]}
          </span>
          <small>v{material.version}</small>
        </div>
      </header>
      <dl className="miyun-metric-grid">
        <div><dt>投放天数</dt><dd>{metric(card?.delivery_days)}</dd></div>
        <div><dt>累计曝光</dt><dd>{metric(card?.cumulative_impressions)}</dd></div>
        <div><dt>关联广告</dt><dd>{metric(card?.related_ads)}</dd></div>
        <div><dt>关联达人</dt><dd>{metric(card?.related_creators, card?.related_creators_known)}</dd></div>
        <div><dt>素材分</dt><dd>{metric(card?.material_score)}</dd></div>
      </dl>
      {preview ? (
        previewFailed && !previewActive ? (
          <div className="miyun-preview-gate">
            <div>
              <b>预览加载失败</b>
              <small>源地址可能已过期或暂时受限；重试会重新加入受控加载队列。</small>
            </div>
            <button type="button" className="secondary-button" onClick={requestPreview}>
              重试预览
            </button>
          </div>
        ) : !previewActive && !previewReady ? (
          <div className="miyun-preview-gate" aria-live="polite">
            <div>
              <b>等待自动加载预览</b>
              <small>页面同时只请求 {previewConcurrency} 条，当前批次会依次加载，无需逐条点击。</small>
            </div>
          </div>
        ) : (
          <div className="miyun-preview-frame" aria-busy={!previewReady}>
            <video
              controls
              preload="metadata"
              src={`${preview}?attempt=${previewAttempt}`}
              aria-label={`${material.title ?? material.miyun_material_id} 的授权预览`}
              onLoadedMetadata={() => {
                if (!previewReady) onPreviewSettled();
                setPreviewReady(true);
              }}
              onError={() => {
                if (!previewFailed) onPreviewSettled();
                setPreviewFailed(true);
              }}
            />
            {!previewReady ? <small role="status">正在安全下载并准备预览…</small> : null}
          </div>
        )
      ) : (
        <small>
          {material.import_method === "crawler"
            ? "服务端受权预览暂不可用。"
            : "此候选没有服务端预览。"}
        </small>
      )}
      <label>
        人工备注
        <textarea
          name={`miyun-material-note-${material.id}`}
          placeholder="记录判断依据或补充说明…"
          value={note}
          onChange={(e) => onNote(e.target.value)}
          maxLength={1000}
        />
      </label>
      <div className="miyun-card-actions">
        {material.selection_status === "discovered" ? (
          <>
            <button type="button" className="secondary-button danger" disabled={busy} onClick={onReject}>
              <X size={14} aria-hidden="true" />
              拒绝候选
            </button>
            <button type="button" className="primary-button" disabled={busy} onClick={onConfirm}>
              <Check size={14} aria-hidden="true" />
              确认并入库
            </button>
          </>
        ) : null}
        {material.selection_status === "confirmed" && ["pending", "failed"].includes(material.import_status) ? (
          <button type="button" className="primary-button" disabled={busy} onClick={onRetry}>
            <RotateCcw size={14} aria-hidden="true" />
            重试导入
          </button>
        ) : null}
      </div>
    </article>
  );
}
