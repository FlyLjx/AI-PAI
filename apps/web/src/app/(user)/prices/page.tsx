'use client';

import Link from 'next/link';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { BadgeDollarSign, Layers3, LoaderCircle, RefreshCw, WalletCards } from 'lucide-react';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { APIError, portalApi, type PricingModel } from '@/lib/portal-api';

type SizeTier = '1k' | '2k' | '4k';

const SIZE_TIERS: SizeTier[] = ['1k', '2k', '4k'];

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '模型价格加载失败';
}

function unitPrice(value: number): string {
  const price = Number(value || 0);
  return price.toLocaleString('zh-CN', { minimumFractionDigits: price % 1 === 0 ? 0 : 2, maximumFractionDigits: 4 });
}

function tierEnabled(model: PricingModel, tier: SizeTier): boolean {
  return (model.enabledSizeTiers || []).includes(tier);
}

export default function ModelPricesPage() {
  const [models, setModels] = useState<PricingModel[]>([]);
  const [creditName, setCreditName] = useState('余额');
  const [search, setSearch] = useState('');
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

  const filteredModels = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return models;
    return models.filter((model) => model.displayName.toLowerCase().includes(keyword));
  }, [models, search]);

  const priceCell = (model: PricingModel, tier: SizeTier) => tierEnabled(model, tier) ? (
    <span className="whitespace-nowrap">
      <strong className="mono block text-[13px] text-[#087443]">{unitPrice(model[`price${tier}`])}</strong>
      <small className="mt-0.5 block text-[10px] text-zinc-400">{creditName} / 张</small>
    </span>
  ) : <span className="text-[11px] text-zinc-300">未开放</span>;

  return (
    <div className="page-stack">
      <PageHeader title="模型价目表" description="查看当前可调用模型及不同清晰度的单张余额价格">
        <Link href="/billing" className="btn"><WalletCards size={14} />计费中心</Link>
        <button className="btn icon" type="button" onClick={() => void load()} disabled={loading} title="刷新价格" aria-label="刷新模型价格">
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
        </button>
      </PageHeader>

      <section className="notice flex flex-col gap-3 sm:flex-row sm:items-center" aria-label="模型计费说明">
        <span className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-[#eaf8ef] text-[#087443]"><BadgeDollarSign size={16} /></span>
        <div className="min-w-0 flex-1"><strong className="block text-xs">按当前后台售价实时展示</strong><p className="mt-0.5 text-[11px] leading-5 text-zinc-500">余额模式按成功返回的图片数量扣费；订阅模式按套餐图片额度扣除，不重复扣账户余额。</p></div>
        <span className="shrink-0 text-[11px] text-zinc-400">价格单位：{creditName} / 张</span>
      </section>

      {error && <div className="notice flex items-center justify-between gap-3" role="alert"><span>{error}</span><button type="button" onClick={() => void load()} className="font-bold text-[#087443]">重试</button></div>}

      {loading ? (
        <div className="section-panel grid min-h-[320px] place-items-center"><div className="text-center"><LoaderCircle size={24} className="mx-auto animate-spin text-[#12B76A]" /><p className="mt-2 text-xs text-zinc-400">正在读取模型价格...</p></div></div>
      ) : (
        <DataTable
          headers={[
            { key: 'model', label: '模型' },
            { key: '1k', label: '1K 单张价格' },
            { key: '2k', label: '2K 单张价格' },
            { key: '4k', label: '4K 单张价格' },
            { key: 'tiers', label: '开放清晰度' },
          ]}
          data={filteredModels}
          searchPlaceholder="搜索模型名称"
          searchValue={search}
          onSearchChange={setSearch}
          filterControls={<span className="text-[11px] text-zinc-400">显示 {filteredModels.length} / {models.length} 个模型</span>}
          emptyState={<EmptyState title={models.length ? '没有匹配的模型' : '暂无可用模型'} description={models.length ? '请更换搜索关键词。' : '后台启用模型并开放清晰度后会显示在这里。'} icon={Layers3} />}
          renderRow={(model) => (
            <tr key={model.id} className="hover:bg-[#fcfdfc]">
              <td className="px-4 py-3"><strong className="block max-w-[260px] truncate text-[13px]">{model.displayName}</strong><code className="mono mt-1 block max-w-[260px] truncate text-[10px] text-zinc-400">{model.id}</code></td>
              {SIZE_TIERS.map((tier) => <td key={tier} className="px-4 py-3">{priceCell(model, tier)}</td>)}
              <td className="px-4 py-3"><div className="flex flex-wrap gap-1.5">{SIZE_TIERS.map((tier) => <span key={tier} className={`rounded border px-2 py-1 text-[10px] font-bold ${tierEnabled(model, tier) ? 'border-[#bbf7d0] bg-[#f0fdf4] text-[#087443]' : 'border-[#edf0ee] bg-[#fafbf9] text-zinc-300'}`}>{tier.toUpperCase()}</span>)}</div></td>
            </tr>
          )}
          renderMobileItem={(model) => (
            <article key={model.id} className="overflow-hidden rounded-md border border-[#dce4df] bg-white shadow-sm">
              <header className="flex items-start justify-between gap-3 border-b border-[#edf0ee] px-4 py-3"><div className="min-w-0"><strong className="block truncate text-sm">{model.displayName}</strong><code className="mono mt-0.5 block truncate text-[10px] text-zinc-400">{model.id}</code></div><span className="status-pill active shrink-0">可调用</span></header>
              <div className="grid grid-cols-3 divide-x divide-[#edf0ee]">{SIZE_TIERS.map((tier) => <div key={tier} className="min-w-0 px-2 py-3 text-center"><small className="block text-[10px] font-bold text-zinc-400">{tier.toUpperCase()}</small>{tierEnabled(model, tier) ? <><strong className="mono mt-1 block truncate text-[13px] text-[#087443]">{unitPrice(model[`price${tier}`])}</strong><span className="mt-0.5 block truncate text-[9px] text-zinc-400">{creditName}/张</span></> : <span className="mt-2 block text-[10px] text-zinc-300">未开放</span>}</div>)}</div>
              <footer className="flex items-center justify-between gap-3 border-t border-[#edf0ee] bg-[#fafbf9] px-4 py-2.5 text-[10px] text-zinc-400"><span>余额按张计费</span><span>{(model.enabledSizeTiers || []).map((tier) => tier.toUpperCase()).join(' / ')}</span></footer>
            </article>
          )}
        />
      )}
    </div>
  );
}
