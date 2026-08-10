'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  CircleCheck,
  Info,
  LoaderCircle,
  RefreshCw,
  Search,
} from 'lucide-react';
import { DataTable } from '@/components/common/DataTable';
import { APIError, portalApi, type PricingModel } from '@/lib/portal-api';

type SizeTier = '1k' | '2k' | '4k';

const SIZE_TIERS: SizeTier[] = ['1k', '2k', '4k'];
const MODEL_PRICE_PAGE_SIZE = 6;
const SIZE_META: Record<SizeTier, { label: string; resolution: string }> = {
  '1k': { label: '1K', resolution: '1024×1024' },
  '2k': { label: '2K', resolution: '2048×2048' },
  '4k': { label: '4K', resolution: '2304×3072' },
};

const MODEL_PRICE_HEADERS = [
  { key: 'model', label: '模型' },
  { key: 'type', label: '类型' },
  { key: 'resolution', label: '分辨率' },
  { key: 'singlePrice', label: '单张价格' },
  { key: 'batchPrice', label: '批量价格' },
  { key: 'status', label: '状态' },
];

type PricingRow = {
  id: string;
  model: string;
  type: '文生图' | '图生图';
  resolution: string;
  tier: SizeTier | null;
  singlePrice: number | null;
  batchPrice: number | null;
  enabled: boolean;
};

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '模型价格加载失败';
}

function unitPrice(value: number | null): string {
  if (value === null || !Number.isFinite(Number(value))) return '--';
  const price = Number(value || 0);
  return price.toLocaleString('zh-CN', {
    minimumFractionDigits: price % 1 === 0 ? 0 : 2,
    maximumFractionDigits: 4,
  });
}

function tierEnabled(model: PricingModel, tier: SizeTier): boolean {
  return (model.enabledSizeTiers || []).includes(tier);
}

function modelType(model: PricingModel): PricingRow['type'] {
  const source = `${model.id} ${model.displayName}`.toLowerCase();
  return /edit|variation|图生图/.test(source) ? '图生图' : '文生图';
}

export default function ModelPricesPage() {
  const [models, setModels] = useState<PricingModel[]>([]);
  const [creditName, setCreditName] = useState('余额');
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    const [modelsResult, settingsResult] = await Promise.allSettled([
      portalApi.pricingModels(),
      portalApi.publicSettings(),
    ]);
    if (modelsResult.status === 'fulfilled') {
      setModels(modelsResult.value.data || []);
    } else {
      setModels([]);
      setError(errorMessage(modelsResult.reason));
    }
    if (settingsResult.status === 'fulfilled') {
      setCreditName(String(settingsResult.value.data.creditName || '余额'));
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const rows = useMemo<PricingRow[]>(() => models.flatMap((model): PricingRow[] => {
    const enabledTiers = SIZE_TIERS.filter((tier) => tierEnabled(model, tier));
    if (enabledTiers.length === 0) {
      return [{
        id: `${model.id}-unavailable`,
        model: model.displayName,
        type: modelType(model),
        resolution: '--',
        tier: null,
        singlePrice: null,
        batchPrice: null,
        enabled: false,
      }];
    }
    return enabledTiers.map((tier) => ({
      id: `${model.id}-${tier}`,
      model: model.displayName,
      type: modelType(model),
      resolution: SIZE_META[tier].resolution,
      tier,
      singlePrice: Number(model[`price${tier}`]),
      batchPrice: Number(model[`price${tier}`]),
      enabled: true,
    }));
  }), [models]);

  const filteredRows = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return rows;
    return rows.filter((row) => `${row.model} ${row.type} ${row.resolution} ${row.tier || ''}`.toLowerCase().includes(keyword));
  }, [rows, search]);

  const totalPages = Math.max(1, Math.ceil(filteredRows.length / MODEL_PRICE_PAGE_SIZE));
  const visiblePage = Math.min(page, totalPages);
  const visibleRows = filteredRows.slice((visiblePage - 1) * MODEL_PRICE_PAGE_SIZE, visiblePage * MODEL_PRICE_PAGE_SIZE);

  return (
    <div className="model-pricing-page page-stack">
      <header className="model-pricing-head model-pricing-toolbar" aria-label="模型价格工具栏">
        <div className="model-pricing-head-actions">
          <label className="model-pricing-search">
            <Search size={14} aria-hidden="true" />
            <input value={search} onChange={(event) => { setSearch(event.target.value); setPage(1); }} placeholder="搜索模型" aria-label="搜索模型" />
          </label>
          <button className="model-pricing-refresh" type="button" onClick={() => void load()} disabled={loading} title="刷新价格" aria-label="刷新模型价格">
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          </button>
        </div>
      </header>

      {error && <div className="model-pricing-error" role="alert"><span>{error}</span><button type="button" onClick={() => void load()}>重试</button></div>}

      <section className="model-pricing-table-card" aria-labelledby="model-pricing-table-title">
        <header className="model-pricing-card-head">
          <div>
            <h2 id="model-pricing-table-title">模型价目</h2>
            <p>当前可调用模型及不同分辨率的图片价格</p>
          </div>
          <span className="model-pricing-count">{filteredRows.length} 个价格项</span>
        </header>

        <DataTable
          embedded
          className="model-pricing-data-table"
          headers={MODEL_PRICE_HEADERS}
          data={loading ? [] : visibleRows}
          currentPage={visiblePage}
          totalPages={totalPages}
          totalItems={filteredRows.length}
          onPageChange={setPage}
          paginationDisabled={loading}
          loading={loading}
          loadingState={<div className="model-pricing-empty"><LoaderCircle size={20} className="animate-spin" /><span>正在读取模型价格...</span></div>}
          emptyState={<div className="model-pricing-empty"><Info size={20} /><strong>{models.length ? '没有匹配的价格项' : '暂无可用模型'}</strong><span>{models.length ? '请更换搜索关键词。' : '后台启用模型并开放清晰度后会显示在这里。'}</span></div>}
          tableWrapClassName="model-pricing-table-wrap"
          tableClassName="model-pricing-table"
          renderRow={(row) => (
            <tr key={row.id}>
              <td className="model-pricing-model"><strong>{row.model}</strong>{row.tier && <small>{SIZE_META[row.tier].label}</small>}</td>
              <td>{row.type}</td>
              <td className="model-pricing-resolution">{row.resolution}</td>
              <td>{row.enabled ? <><strong className="model-pricing-price">¥{unitPrice(row.singlePrice)}</strong><small>{creditName} / 张</small></> : <span className="model-pricing-muted">未开放</span>}</td>
              <td>{row.enabled ? <><strong className="model-pricing-price">¥{unitPrice(row.batchPrice)}</strong><small>{creditName} / 张</small></> : <span className="model-pricing-muted">未开放</span>}</td>
              <td><span className={`model-pricing-status ${row.enabled ? '' : 'is-disabled'}`}><i />{row.enabled ? '可用' : '未开放'}</span></td>
            </tr>
          )}
        />
      </section>

      <section className="model-pricing-notes" aria-labelledby="model-pricing-notes-title">
        <div className="model-pricing-notes-icon"><CircleCheck size={17} /></div>
        <div>
          <h2 id="model-pricing-notes-title">价格说明</h2>
          <ol>
            <li>单张价格：按成功返回的图片数量计费。</li>
            <li>批量请求：仍按图片张数计费，价格与单张保持一致。</li>
            <li>实际费用以调用时的模型、分辨率和返回结果为准。</li>
          </ol>
        </div>
      </section>
    </div>
  );
}
