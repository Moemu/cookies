import { readdir, readFile, stat } from "node:fs/promises";
import { existsSync } from "node:fs";
import { basename, join, resolve } from "node:path";
import { DomainError } from "./errors.js";

const DEFAULT_DATA_DIR = resolve(process.cwd(), "data/insights/public_data_insight_source_export");
const NUMBER_FIELDS = new Set([
  "vv_all",
  "like_cnt_all",
  "comment_cnt_all",
  "share_cnt_all",
  "favourite_cnt_all",
  "finish_vv_all",
]);
const DETAIL_FIELDS = [
  "storyboard_structure",
  "ai_creative_type",
  "item_asr",
  "item_ocr",
  "first3s_visual_creative_type",
  "main_visual_elements",
  "shooting_scene",
  "characters_relation",
  "mentioned_brand",
  "oral_product_desc",
  "bgm_style",
  "bgm_bpm",
  "bgm_emotion",
  "voice_type",
  "speech_speed",
  "oral_script",
  "storyboard_prompt",
  "visual_style",
  "creative_highlight",
] as const;
const LIST_FIELDS = [
  "item_id",
  "url",
  "frame_first",
  "item_title",
  "item_create_day",
  "author_cert_type",
  "vv_all",
  "like_cnt_all",
  "comment_cnt_all",
  "share_cnt_all",
  "favourite_cnt_all",
  "finish_vv_all",
  "ctr",
  "bounce_rate_map",
  "has_ai_generated",
  "industry",
  "date",
  "finish_rate",
  "like_rate",
  "playback_url",
] as const;

type PublicInsightRow = Record<string, string | number | unknown[]>;

export interface PublicInsightQuery {
  page: number;
  pageSize: number;
  keyword?: string;
  industry?: string;
  aiGenerated?: string;
  visualStyle?: string;
  dateFrom?: string;
  dateTo?: string;
  sortBy?: string;
  sortOrder?: string;
}

export async function publicInsightOverview() {
  const store = await loadPublicInsightStore();
  const totalViews = sum(store.rows, "vv_all");
  const totalLikes = sum(store.rows, "like_cnt_all");
  const totalFinishes = sum(store.rows, "finish_vv_all");
  const industries = new Map<string, { count: number; views: number }>();
  for (const row of store.rows) {
    const industry = text(row.industry) || "未分类";
    const current = industries.get(industry) ?? { count: 0, views: 0 };
    current.count += 1;
    current.views += number(row.vv_all);
    industries.set(industry, current);
  }
  return {
    total_videos: store.rows.length,
    total_views: totalViews,
    average_like_rate: totalViews ? totalLikes / totalViews : 0,
    average_finish_rate: totalViews ? totalFinishes / totalViews : 0,
    ai_ratio: store.rows.length ? store.rows.filter((row) => text(row.has_ai_generated) === "是").length / store.rows.length : 0,
    industries: [...industries.entries()]
      .map(([name, values]) => ({ name, ...values }))
      .sort((left, right) => right.views - left.views),
    files: store.files,
    loaded_at: store.loadedAt,
    data_dir: store.root,
  };
}

export async function publicInsightFilters() {
  const store = await loadPublicInsightStore();
  const industries = countBy(store.rows, (row) => text(row.industry) || "未分类");
  const visualStyles = new Map<string, number>();
  for (const row of store.rows) {
    for (const style of splitLabels(text(row.visual_style))) {
      visualStyles.set(style, (visualStyles.get(style) ?? 0) + 1);
    }
  }
  const dates = store.rows.map((row) => text(row.date)).filter(Boolean).sort();
  return {
    industries: options(industries),
    visual_styles: options(visualStyles).slice(0, 30),
    ai_types: ["全部", "否", "是"],
    date_range: { min: dates[0] ?? "", max: dates.at(-1) ?? "" },
  };
}

export async function queryPublicInsightVideos(query: PublicInsightQuery) {
  const store = await loadPublicInsightStore();
  const keyword = (query.keyword ?? "").trim().toLowerCase();
  const sortBy = query.sortBy && LIST_FIELDS.includes(query.sortBy as typeof LIST_FIELDS[number])
    ? query.sortBy
    : "vv_all";
  const sortOrder = query.sortOrder === "asc" ? "asc" : "desc";
  const filtered = store.rows.filter((row) => {
    const searchable = [
      row.item_title,
      row.item_asr,
      row.item_ocr,
      row.creative_highlight,
      row.mentioned_brand,
      row.industry,
    ].map(text).join(" ").toLowerCase();
    if (keyword && !searchable.includes(keyword)) return false;
    if (query.industry && (text(row.industry) || "未分类") !== query.industry) return false;
    if (query.aiGenerated && query.aiGenerated !== "全部" && text(row.has_ai_generated) !== query.aiGenerated) return false;
    if (query.visualStyle && !text(row.visual_style).includes(query.visualStyle)) return false;
    if (query.dateFrom && text(row.date) < query.dateFrom.replaceAll("-", "")) return false;
    if (query.dateTo && text(row.date) > query.dateTo.replaceAll("-", "")) return false;
    return true;
  }).sort((left, right) => {
    const leftValue = comparable(left[sortBy]);
    const rightValue = comparable(right[sortBy]);
    return sortOrder === "asc" ? leftValue - rightValue : rightValue - leftValue;
  });
  const page = Math.max(1, query.page || 1);
  const pageSize = Math.min(Math.max(1, query.pageSize || 20), 100);
  const start = (page - 1) * pageSize;
  return {
    items: filtered.slice(start, start + pageSize).map((row) => pick(row, LIST_FIELDS)),
    total: filtered.length,
    page,
    page_size: pageSize,
    pages: filtered.length ? Math.ceil(filtered.length / pageSize) : 0,
  };
}

export async function getPublicInsightVideo(itemId: string) {
  const store = await loadPublicInsightStore();
  const row = store.rows.find((candidate) => text(candidate.item_id) === itemId);
  if (!row) throw new DomainError("NOT_FOUND", "未找到该示例视频洞察");
  return {
    ...pick(row, LIST_FIELDS),
    ...pick(row, DETAIL_FIELDS),
    storyboard: parseStoryboard(text(row.storyboard_structure)),
    source_file: text(row.source_file),
  };
}

async function loadPublicInsightStore() {
  const root = process.env.PUBLIC_INSIGHT_DATA_DIR ?? DEFAULT_DATA_DIR;
  const csvFiles = await listCsvFiles(root);
  const rows: PublicInsightRow[] = [];
  const files = [];
  const seen = new Set<string>();
  for (const path of csvFiles) {
    const content = await readFile(path, "utf-8");
    const records = parseCsv(content);
    const [headers = [], ...body] = records;
    let rowCount = 0;
    for (const values of body) {
      const raw = Object.fromEntries(headers.map((header, index) => [header, values[index] ?? ""]));
      const itemId = cleanText(raw.item_id);
      if (!itemId || seen.has(itemId)) continue;
      seen.add(itemId);
      rows.push(normalizeRow(raw, root, basename(path) || "unknown.csv"));
      rowCount += 1;
    }
    const fileStat = await stat(path);
    files.push({
      filename: basename(path) || "unknown.csv",
      row_count: rowCount,
      modified_at: fileStat.mtime.toISOString(),
    });
  }
  return { root, rows, files, loadedAt: new Date().toISOString() };
}

async function listCsvFiles(root: string): Promise<string[]> {
  try {
    const entries = await readdir(root);
    return entries.filter((entry) => entry.toLowerCase().endsWith(".csv")).sort().map((entry) => join(root, entry));
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return [];
    throw error;
  }
}

function normalizeRow(raw: Record<string, string>, root: string, sourceFile: string): PublicInsightRow {
  const row: PublicInsightRow = {};
  for (const [key, value] of Object.entries(raw)) {
    row[key] = NUMBER_FIELDS.has(key) ? cleanNumber(value) : cleanText(value);
  }
  const views = number(row.vv_all);
  row.finish_rate = views ? number(row.finish_vv_all) / views : 0;
  row.like_rate = views ? number(row.like_cnt_all) / views : 0;
  row.source_file = sourceFile;
  const itemId = text(row.item_id);
  const localVideo = join(root, "downloads", "douyin", `${itemId}.mp4`);
  row.playback_url = existsSync(localVideo) ? `/api/public-insights/media/${encodeURIComponent(itemId)}.mp4` : "";
  return row;
}

function parseCsv(content: string): string[][] {
  const rows: string[][] = [];
  let row: string[] = [];
  let field = "";
  let quoted = false;
  for (let index = 0; index < content.length; index += 1) {
    const char = content[index];
    const next = content[index + 1];
    if (char === "\"") {
      if (quoted && next === "\"") {
        field += "\"";
        index += 1;
      } else {
        quoted = !quoted;
      }
      continue;
    }
    if (char === "," && !quoted) {
      row.push(field);
      field = "";
      continue;
    }
    if ((char === "\n" || char === "\r") && !quoted) {
      if (char === "\r" && next === "\n") index += 1;
      row.push(field);
      if (row.some((value) => value.length)) rows.push(row);
      row = [];
      field = "";
      continue;
    }
    field += char;
  }
  row.push(field);
  if (row.some((value) => value.length)) rows.push(row);
  return rows;
}

function cleanText(value: unknown): string {
  if (value === undefined || value === null || value === "NULL" || value === "null" || value === "None") return "";
  return String(value).trim();
}

function cleanNumber(value: unknown): number {
  const parsed = Number(cleanText(value));
  return Number.isFinite(parsed) ? parsed : 0;
}

function parseStoryboard(value: string): unknown[] {
  if (!value) return [];
  try {
    const parsed = JSON.parse(value) as unknown;
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function text(value: unknown): string {
  return typeof value === "string" ? value : value === undefined || value === null ? "" : String(value);
}

function number(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : cleanNumber(value);
}

function comparable(value: unknown): number {
  if (typeof value === "number") return value;
  const parsed = Number.parseFloat(text(value).replace("%", ""));
  return Number.isFinite(parsed) ? parsed : 0;
}

function sum(rows: PublicInsightRow[], field: string): number {
  return rows.reduce((total, row) => total + number(row[field]), 0);
}

function splitLabels(value: string): string[] {
  return value.replaceAll("、", "|").replaceAll("，", "|").replaceAll("/", "|").split("|").map((item) => item.trim()).filter(Boolean);
}

function countBy(rows: PublicInsightRow[], selector: (row: PublicInsightRow) => string): Map<string, number> {
  const counts = new Map<string, number>();
  for (const row of rows) {
    const key = selector(row);
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return counts;
}

function options(counts: Map<string, number>) {
  return [...counts.entries()].map(([value, count]) => ({ value, count })).sort((left, right) => right.count - left.count);
}

function pick<T extends readonly string[]>(row: PublicInsightRow, fields: T): Record<T[number], string | number | unknown[]> {
  return Object.fromEntries(fields.map((field) => [field, row[field] ?? ""])) as Record<T[number], string | number | unknown[]>;
}
