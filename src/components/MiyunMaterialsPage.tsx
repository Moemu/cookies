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
const idempotencyKey = () =>
  globalThis.crypto?.randomUUID?.() ?? `miyun-${Date.now()}`;
const miyunDocumentFile = /\.(pdf|docx|md)$/i;
const miyunProjectMediaFile = /\.(png|jpe?g|webp|mp4)$/i;

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
  const [notice, setNotice] = useState("");
  const [productId, setProductId] = useState("");
  const [productName, setProductName] = useState("");
  const [categoryName, setCategoryName] = useState("");
  const [draft, setDraft] = useState<ApiMiyunProductProfile | null>(null);
  const [profileId, setProfileId] = useState("");
  const [handoffMaterialIds, setHandoffMaterialIds] = useState<string[]>([]);
  const [handoffProfileId, setHandoffProfileId] = useState("");
  const [note, setNote] = useState("");
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<CardField>("material_score");
  const uploadInputRef = useRef<HTMLInputElement>(null);
  const automaticallyVerifiedProjectRef = useRef<string | null>(null);
  const load = useCallback(async (verifyConnection = false) => {
    if (!currentProject.id) return;
    const shouldVerifyConnection =
      verifyConnection &&
      automaticallyVerifiedProjectRef.current !== currentProject.id;
    if (shouldVerifyConnection)
      automaticallyVerifiedProjectRef.current = currentProject.id;
    setLoadState("loading");
    setNotice("");
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
        materialPage,
        mediaAssets,
        documentPage,
        source,
        handoffPage,
      ] = await Promise.all([
        api.listMiyunProductProfiles(currentProject.id),
        api.listMiyunCrawlJobs(currentProject.id),
        api.listMiyunMaterials(currentProject.id),
        api.listProjectMediaAssets(currentProject.id),
        api.listKnowledgeDocuments(currentProject.id),
        api.getMiyunProductSource(currentProject.id),
        api.listMiyunHandoffs(currentProject.id),
      ]);
      const details = await Promise.all(
        materialPage.items.map(async (item) => {
          const detail = await api.getMiyunMaterial(currentProject.id, item.id);
          return { ...detail.material, snapshots: detail.snapshots };
        }),
      );
      if (
        shouldVerifyConnection &&
        nextConnection &&
        nextConnection.status !== "disabled"
      ) {
        try {
          nextConnection = await api.verifyMiyunConnection(
            currentProject.id,
            nextConnection.version,
          );
        } catch (error) {
          setNotice(`自动验证未完成：${errorText(error)}`);
        }
      }
      setConnection(nextConnection);
      setProfiles(profilePage.items);
      setJobs(jobPage.items);
      setHandoffs(handoffPage.items);
      setMaterials(details);
      setAssets(mediaAssets);
      setDocuments(documentPage.items);
      setProductSource(source);
      if (source.products.length === 1) {
        setProductId(source.products[0].id);
        setProductName(source.products[0].name);
      }
      setCategoryName(source.category_name);
      setLoadState("ready");
    } catch (error) {
      setLoadState(
        error instanceof ApiRequestError && error.status === 403
          ? "forbidden"
          : "error",
      );
      setNotice(errorText(error));
    }
  }, [currentProject.id]);
  useEffect(() => {
    void load(true);
  }, [load]);
  useEffect(() => {
    setNotice("");
    setDraft(null);
    setNote("");
    setSearch("");
    setAssetRefs([]);
    setDocumentIds([]);
    setHandoffMaterialIds([]);
    setHandoffProfileId("");
  }, [currentProject.id, view]);
  const run = async (work: () => Promise<unknown>, message: string) => {
    setBusy(true);
    setNotice("");
    try {
      await work();
      setNotice(message);
      await load();
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
  const visible = useMemo(
    () =>
      sortMiyunMaterials(
        materials.filter((item) =>
          `${item.title ?? ""} ${item.miyun_material_id}`
            .toLowerCase()
            .includes(search.toLowerCase()),
        ),
        sort,
      ),
    [materials, search, sort],
  );
  const hasSelectedSources = assetRefs.length > 0 || documentIds.length > 0;
  const selectedAssets = assets.filter((asset) => assetRefs.includes(asset.id));
  const selectedDocuments = documents.filter((document) =>
    documentIds.includes(document.id),
  );
  const handoffMaterials = materials.filter(
    (material) =>
      material.selection_status === "confirmed" &&
      (material.import_status === "imported" ||
        material.import_status === "deduplicated"),
  );
  const handoffProfiles = profiles.filter(
    (profile) => profile.status === "confirmed",
  );
  const selectedHandoffMaterials = handoffMaterials.filter((material) =>
    handoffMaterialIds.includes(material.id),
  );
  const selectedHandoffProfile =
    handoffProfiles.find((profile) => profile.id === handoffProfileId);
  const toggle = (
    set: React.Dispatch<React.SetStateAction<string[]>>,
    id: string,
  ) =>
    set((current) =>
      current.includes(id)
        ? current.filter((value) => value !== id)
        : [...current, id],
    );
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
        <header className="core-flow-toolbar">
          <div>
            <span className="section-label">MIYUN MATERIAL INSIGHTS</span>
            <h2>
              {view === "analysis"
                ? "产品分析"
                : view === "jobs"
                  ? "采集任务"
                  : view === "materials"
                    ? "素材候选"
                    : "裂变任务"}
            </h2>
            <p>
              当前 Project：{currentProject.name}
              。项目切换会隔离并重新读取数据。
            </p>
          </div>
          <button
            className="secondary-button"
            disabled={busy || loadState === "loading"}
            onClick={() => void load()}
          >
            <RefreshCw size={15} />
            刷新
          </button>
        </header>
        {view === "fission" ? (
          <>
            <FissionTask
              busy={busy}
              loadState={loadState}
              notice={notice}
              materials={handoffMaterials}
              profiles={handoffProfiles}
              handoffs={handoffs}
              selectedMaterialIds={selectedHandoffMaterials.map((material) => material.id)}
              selectedProfileId={selectedHandoffProfile?.id ?? ""}
              projectId={currentProject.id}
              onMaterialChange={(id) => toggle(setHandoffMaterialIds, id)}
              onProfileChange={setHandoffProfileId}
              onCreate={() =>
                run(
                  () => api.createMiyunHandoff(currentProject.id, {
                    source_material_ids: selectedHandoffMaterials.map((material) => material.id),
                    product_profile_id: selectedHandoffProfile!.id,
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
              onRetry={() => void load()}
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
                    <span>米云连接</span>
                    <b>{connection ? miyunStateCopy(connection.status, "connection").title : "尚未配置连接"}</b>
                    <small>{connection ? miyunStateCopy(connection.status, "connection").action : "请在系统设置中保存 Cookie 并验证连接。"}</small>
                  </div>
                  <a className="secondary-button" href="/settings">前往系统设置</a>
                </section>
              <section className="miyun-grid">
                <article className="surface-card">
                  <h3>分析产品</h3>
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
                <article className="surface-card">
                  <h3>产品分析草稿</h3>
                  {!profiles.length && !draft ? (
                    <div className="panel-empty">
                      暂无草稿。请先验证连接并分析产品。
                    </div>
                  ) : (
                    (draft ? [draft] : profiles).map((profile) => (
                      <button
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
                        <b>{profile.product_name}</b>
                        <small>
                          {profile.status} · v{profile.version}
                        </small>
                      </button>
                    ))
                  )}
                </article>
                {draft ? (
                  <ProfileEditor
                    draft={draft}
                    busy={busy}
                    onChange={setDraft}
                    onConfirm={() =>
                      void run(
                        () =>
                          api.confirmMiyunProductProfile(
                            currentProject.id,
                            draft.id,
                            draft.version,
                            profileQuery(draft),
                          ),
                        "查询已由人工确认。",
                      )
                    }
                  />
                ) : null}
              </section>
              </>
            ) : null}
            {loadState === "ready" && view === "jobs" ? (
              <section className="miyun-grid">
                <article className="surface-card">
                  <h3>创建采集任务</h3>
                  <select
                    aria-label="产品分析"
                    value={selected?.id ?? ""}
                    onChange={(e) => setProfileId(e.target.value)}
                  >
                    {profiles
                      .filter((profile) => profile.status === "confirmed")
                      .map((profile) => (
                        <option key={profile.id} value={profile.id}>
                          {profile.product_name}
                        </option>
                      ))}
                  </select>
                  <button
                    disabled={
                      busy ||
                      !selected ||
                      selected.status !== "confirmed" ||
                      connection?.status !== "ready"
                    }
                    onClick={() =>
                      void run(
                        () =>
                          api.createMiyunCrawlJob(
                            currentProject.id,
                            {
                              product_profile_id: selected!.id,
                              operation: "product",
                            },
                            idempotencyKey(),
                          ),
                        "采集任务已创建。",
                      )
                    }
                  >
                    按产品采集
                  </button>
                </article>
                <article className="surface-card">
                  <h3>采集进度</h3>
                  {!jobs.length ? (
                    <div className="panel-empty">
                      暂无采集任务。请先人工确认产品查询条件，再明确创建采集任务。
                    </div>
                  ) : (
                    jobs.map((job) => (
                      <div key={job.id}>
                        <b>
                          {job.operation} · {miyunStateCopy(job.status).title}
                        </b>
                        <small>
                          页数 {job.completed_pages} · 发现{" "}
                          {job.discovered_count} · 去重 {job.deduplicated_count}{" "}
                          · 下载 {job.downloaded_count} · 失败{" "}
                          {job.failed_count}
                        </small>
                        <small>{miyunStateCopy(job.status).action}</small>
                        {miyunCrawlErrorCopy(job) ? (
                          <small>{miyunCrawlErrorCopy(job)}</small>
                        ) : null}
                        {job.status === "cooling_down" && job.cooldown_until ? (
                          <small>冷却至：{job.cooldown_until}</small>
                        ) : null}
                        {["queued", "running", "cooling_down"].includes(
                          job.status,
                        ) ? (
                          <button
                            disabled={busy || job.status === "cooling_down"}
                            onClick={() =>
                              void run(
                                () =>
                                  api.cancelMiyunCrawlJob(
                                    currentProject.id,
                                    job.id,
                                    job.version,
                                  ),
                                "已请求取消任务。",
                              )
                            }
                          >
                            取消
                          </button>
                        ) : null}
                        {["failed", "cancelled", "partial"].includes(
                          job.status,
                        ) ? (
                          <button
                            disabled={busy}
                            onClick={() =>
                              void run(
                                () =>
                                  api.retryMiyunCrawlJob(
                                    currentProject.id,
                                    job.id,
                                    idempotencyKey(),
                                  ),
                                "已创建重试任务。",
                              )
                            }
                          >
                            <RotateCcw size={14} />
                            重试
                          </button>
                        ) : null}
                      </div>
                    ))
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
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                    />
                  </div>
                  <label>
                    米云累计口径：来自每条候选最新数据卡；未知值不参与数值排序。
                    <select
                      aria-label="按数据卡排序"
                      value={sort}
                      onChange={(e) => setSort(e.target.value as CardField)}
                    >
                      <option value="delivery_days">投放天数</option>
                      <option value="cumulative_impressions">累计曝光</option>
                      <option value="related_ads">关联广告</option>
                      <option value="related_creators">关联达人</option>
                      <option value="material_score">素材分</option>
                    </select>
                  </label>
                </div>
                {!visible.length ? (
                  <div className="panel-empty">
                    <b>{miyunStateCopy("empty", "materials").title}</b>
                    <small>请先完成采集；也可清除搜索条件后重新查看。</small>
                  </div>
                ) : null}
                {visible.map((material) => (
                  <MaterialCard
                    key={material.id}
                    material={material}
                    note={note}
                    busy={busy}
                    onNote={setNote}
                    onConfirm={() =>
                      void run(
                        () =>
                          api.confirmMiyunMaterial(
                            currentProject.id,
                            material.id,
                            material.version,
                            note,
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
                            note,
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
              </section>
            ) : null}
          </>
        )}
      </div>
    </StateBoundary>
  );
}
function FissionTask({
  busy,
  loadState,
  notice,
  materials,
  profiles,
  handoffs,
  selectedMaterialIds,
  selectedProfileId,
  projectId,
  onMaterialChange,
  onProfileChange,
  onCreate,
  onMarkDelivered,
  onRetry,
}: {
  busy: boolean;
  loadState: "loading" | "ready" | "error" | "forbidden";
  notice: string;
  materials: MaterialDetail[];
  profiles: ApiMiyunProductProfile[];
  handoffs: ApiMiyunHandoff[];
  selectedMaterialIds: string[];
  selectedProfileId: string;
  projectId: string;
  onMaterialChange: (id: string) => void;
  onProfileChange: (id: string) => void;
  onCreate: () => Promise<void>;
  onMarkDelivered: (handoff: ApiMiyunHandoff) => Promise<void>;
  onRetry: () => void;
}) {
  const selectedProfile = profiles.find((item) => item.id === selectedProfileId);
  // Older persisted profiles can contain JSON null for optional source lists.
  // Normalize at the UI boundary so a valid confirmed profile never crashes
  // the handoff selector while its frozen file counts are rendered.
  const profile = selectedProfile && {
    ...selectedProfile,
    product_asset_refs: selectedProfile.product_asset_refs ?? [],
    knowledge_document_ids: selectedProfile.knowledge_document_ids ?? [],
  };
  return (
    <section className="miyun-grid" aria-label="裂变任务">
      <article className="surface-card">
        <span className="section-label">VERSIONED HANDOFF</span>
        <h3>创建裂变交接</h3>
        <p>只允许已确认、已入库的本 Project 素材和已确认产品 profile；不会调用外部 AI 或自动交付。</p>
        {loadState !== "ready" ? (
          <div className="panel-empty">
            {notice || miyunStateCopy(loadState).title}
            {loadState !== "loading" ? <button type="button" onClick={onRetry}>刷新</button> : null}
          </div>
        ) : (
          <>
            <label>
              已确认且已入库的爆款素材
              <span role="group" aria-label="handoff-source-materials">
                {materials.map((material) => <label key={material.id}><input type="checkbox" checked={selectedMaterialIds.includes(material.id)} onChange={() => onMaterialChange(material.id)} />{material.title ?? material.miyun_material_id} · {material.import_status} · v{material.version}</label>)}
              </span>
            </label>
            <label>
              已确认产品 profile
              <select aria-label="选择已确认产品 profile" value={selectedProfileId} onChange={(event) => onProfileChange(event.target.value)}>
                <option value="">请选择产品 profile</option>
                {profiles.map((item) => <option key={item.id} value={item.id}>{item.product_name} · confirmed · v{item.version}</option>)}
              </select>
            </label>
            {profile ? <div className="miyun-handoff-profile-files" role="note"><b>将冻结的产品资料</b><small>媒体 {profile.product_asset_refs.length} 项；文档 {profile.knowledge_document_ids.length} 项。资料版本来自已确认 profile，后续修改不会改变此交接。</small></div> : null}
            {!materials.length || !profiles.length ? <small>{!materials.length ? "暂无已确认且已入库/去重的爆款素材。" : "暂无已确认的产品 profile。"}</small> : null}
            <button className="primary-button" disabled={busy || !selectedMaterialIds.length || !selectedProfileId} onClick={() => void onCreate()}>创建交接</button>
          </>
        )}
      </article>
      <article className="surface-card">
        <h3>交接历史</h3>
        <p>“已导出”仅表示 zip 已成功生成并流出；“已交付”必须由人明确确认。</p>
        {!handoffs.length ? <div className="panel-empty">暂无交接记录。</div> : handoffs.map((handoff) => (
          <div className="miyun-handoff-history-row" key={handoff.id}>
            <b>{handoff.id}</b>
            <small>{handoff.status} · manifest {handoff.manifest_version} · 参数 {handoff.parameter_version} · v{handoff.version}</small>
            <small>输入哈希：{handoff.input_hash}</small>
            {handoff.status === "exporting" || handoff.status === "exported" || handoff.status === "delivered" ? <a className="secondary-button" href={api.getMiyunHandoffExportUrl(projectId, handoff.id)} download>{handoff.status === "exporting" ? "导出并下载交接 zip" : "下载交接 zip"}</a> : null}
            {handoff.status === "exported" ? <button type="button" disabled={busy} onClick={() => void onMarkDelivered(handoff)}>人工确认已交付</button> : null}
            {handoff.status === "exporting" ? <small>等待导出：点击“导出并下载交接 zip”开始流式导出；成功后才可人工确认交付。</small> : null}
            {handoff.status === "failed" ? <small>导出失败；请重新下载，失败不会标记为已导出或已交付。</small> : null}
          </div>
        ))}
      </article>
    </section>
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
  return (
    <article className="surface-card">
      <h3>人工编辑并确认查询</h3>
      <label>
        关键词（逗号分隔）
        <input
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
      <button
        disabled={busy || draft.status !== "draft" || !draft.keywords.length}
        onClick={onConfirm}
      >
        确认查询条件
      </button>
    </article>
  );
}
function MaterialCard({
  material,
  note,
  busy,
  onNote,
  onConfirm,
  onReject,
  onRetry,
  preview,
}: {
  material: MaterialDetail;
  note: string;
  busy: boolean;
  onNote: (value: string) => void;
  onConfirm: () => void;
  onReject: () => void;
  onRetry: () => void;
  preview?: string;
}) {
  const card = latestMiyunCard(material);
  return (
    <article className="surface-card">
      <h3>{material.title ?? material.miyun_material_id}</h3>
      <small>
        候选状态：{material.selection_status} · 导入状态：
        {material.import_status} · v{material.version}
      </small>
      <div>
        <span>投放天数 {metric(card?.delivery_days)}</span>
        <span>累计曝光 {metric(card?.cumulative_impressions)}</span>
        <span>关联广告 {metric(card?.related_ads)}</span>
        <span>
          关联达人{" "}
          {metric(card?.related_creators, card?.related_creators_known)}
        </span>
        <span>素材分 {metric(card?.material_score)}</span>
      </div>
      {preview ? (
        <video
          controls
          preload="metadata"
          src={preview}
          aria-label={`${material.title ?? material.miyun_material_id} 的受权预览`}
        />
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
          value={note}
          onChange={(e) => onNote(e.target.value)}
          maxLength={1000}
        />
      </label>
      <button
        disabled={busy || material.selection_status !== "discovered"}
        onClick={onConfirm}
      >
        <Check size={14} />
        人工确认
      </button>
      <button
        disabled={busy || material.selection_status !== "discovered"}
        onClick={onReject}
      >
        <X size={14} />
        拒绝
      </button>
      {material.selection_status === "confirmed" && ["pending", "failed"].includes(material.import_status) ? (
        <button disabled={busy} onClick={onRetry}>
          重试导入
        </button>
      ) : null}
    </article>
  );
}
