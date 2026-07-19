'use client';

/* eslint-disable @next/next/no-img-element */

import Link from 'next/link';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  AlertCircle,
  Download,
  Expand,
  Image as ImageIcon,
  ImagePlus,
  Images,
  KeyRound,
  LoaderCircle,
  MailWarning,
  RefreshCw,
  Sparkles,
  Trash2,
  X,
} from 'lucide-react';
import { toast } from 'sonner';
import { AppSelect, type AppSelectOption } from '@/components/common/AppSelect';
import { PageHeader } from '@/components/common/PageHeader';
import {
  APIError,
  getSession,
  portalApi,
  refreshSession,
  type APIKey,
  type CompatibleModel,
  type ImageGenerationInput,
  type PortalUser,
} from '@/lib/portal-api';

type SizeTier = ImageGenerationInput['size_tier'];
type AspectRatio = ImageGenerationInput['aspect_ratio'];
type OutputFormat = ImageGenerationInput['output_format'];
type ReferenceImage = { id: string; file: File; previewUrl: string };

const maxReferenceImages = 4;
const maxReferenceImageBytes = 20 * 1024 * 1024;
const maxReferenceImagesTotalBytes = 75 * 1024 * 1024;
const supportedReferenceImageTypes = new Set(['image/jpeg', 'image/png', 'image/webp']);

const sizeOptions: AppSelectOption[] = [
  { value: '1k', label: '1K · 标准' },
  { value: '2k', label: '2K · 高清' },
  { value: '4k', label: '4K · 超清' },
];

const ratioOptions: AppSelectOption[] = [
  { value: '1:1', label: '1:1 · 方形' },
  { value: '16:9', label: '16:9 · 横版' },
  { value: '9:16', label: '9:16 · 竖版' },
  { value: '4:3', label: '4:3 · 横版' },
  { value: '3:4', label: '3:4 · 竖版' },
  { value: '3:2', label: '3:2 · 横版' },
  { value: '2:3', label: '2:3 · 竖版' },
];

const quantityOptions: AppSelectOption[] = [1, 2, 3, 4].map((value) => ({
  value: String(value),
  label: `${value} 张`,
}));

const formatOptions: AppSelectOption[] = [
  { value: 'jpeg', label: 'JPEG' },
  { value: 'png', label: 'PNG · 透明背景' },
  { value: 'webp', label: 'WEBP' },
];

const allSizeTiers: SizeTier[] = ['1k', '2k', '4k'];

function sizeTiersForModel(model?: CompatibleModel): SizeTier[] {
  const enabled = (model?.enabled_size_tiers || []).filter((tier): tier is SizeTier => (
    allSizeTiers.includes(tier)
  ));
  return enabled.length > 0 ? enabled : allSizeTiers;
}

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '请求失败，请稍后重试';
}

function formatGeneratedAt(created: number): string {
  if (!created) return '';
  return new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(new Date(created * 1000));
}

function fileExtension(format: OutputFormat): string {
  return format === 'jpeg' ? 'jpg' : format;
}

function taskImageDownloadURL(url: string, filename: string): string {
  const target = new URL(url, window.location.origin);
  if (target.origin !== window.location.origin || !/^\/api\/tasks\/[^/]+\/images\/\d+\/?$/.test(target.pathname)) {
    throw new Error('站内下载地址不可用');
  }
  target.pathname = `${target.pathname.replace(/\/$/, '')}/download`;
  target.search = '';
  target.searchParams.set('filename', filename);
  return `${target.pathname}${target.search}`;
}

export default function PlaygroundPage() {
  const mountedRef = useRef(true);
  const keyRequestRef = useRef(0);
  const generationFormRef = useRef<HTMLFormElement>(null);
  const referenceInputRef = useRef<HTMLInputElement>(null);
  const referenceImagesRef = useRef<ReferenceImage[]>([]);
  const referenceImageIDRef = useRef(0);
  const [user, setUser] = useState<PortalUser | null>(null);
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [selectedKeyId, setSelectedKeyId] = useState('');
  const [apiSecret, setApiSecret] = useState('');
  const [models, setModels] = useState<CompatibleModel[]>([]);
  const [selectedModel, setSelectedModel] = useState('');
  const [prompt, setPrompt] = useState('');
  const [referenceImages, setReferenceImages] = useState<ReferenceImage[]>([]);
  const [referenceDragActive, setReferenceDragActive] = useState(false);
  const [sizeTier, setSizeTier] = useState<SizeTier>('1k');
  const [aspectRatio, setAspectRatio] = useState<AspectRatio>('1:1');
  const [quantity, setQuantity] = useState(1);
  const [outputFormat, setOutputFormat] = useState<OutputFormat>('jpeg');
  const [loadingKeys, setLoadingKeys] = useState(true);
  const [loadingModels, setLoadingModels] = useState(false);
  const [generating, setGenerating] = useState(false);
  const [loadError, setLoadError] = useState('');
  const [modelError, setModelError] = useState('');
  const [generationError, setGenerationError] = useState('');
  const [results, setResults] = useState<string[]>([]);
  const [createdAt, setCreatedAt] = useState(0);
  const [loadedImages, setLoadedImages] = useState<Set<number>>(() => new Set());
  const [failedImages, setFailedImages] = useState<Set<number>>(() => new Set());
  const [downloadingIndex, setDownloadingIndex] = useState<number | null>(null);
  const [previewImage, setPreviewImage] = useState('');
  const [generationFormHeight, setGenerationFormHeight] = useState(0);

  const loadModelsForKey = useCallback(async (current: PortalUser, keyId: string) => {
    const requestId = ++keyRequestRef.current;
    setLoadingModels(true);
    setModelError('');
    setApiSecret('');
    setModels([]);
    setSelectedModel('');

    try {
      const revealed = await portalApi.revealKey(current, keyId);
      const secret = String(revealed.data.key || '');
      if (!secret) throw new Error('服务端未返回 API Key 明文');
      const response = await portalApi.compatibleModels(secret);
      if (!mountedRef.current || requestId !== keyRequestRef.current) return;
      const availableModels = (response.data || []).filter((model) => model.id);
      const firstModel = availableModels[0];
      setApiSecret(secret);
      setModels(availableModels);
      setSelectedModel(firstModel?.id || '');
      setSizeTier(sizeTiersForModel(firstModel)[0]);
      if (availableModels.length === 0) setModelError('当前没有可用的生图模型');
    } catch (error) {
      if (!mountedRef.current || requestId !== keyRequestRef.current) return;
      setModelError(errorMessage(error));
    } finally {
      if (mountedRef.current && requestId === keyRequestRef.current) setLoadingModels(false);
    }
  }, []);

  const loadKeys = useCallback(async () => {
    const current = getSession();
    if (!current) {
      setLoadError('登录状态已失效，请重新登录');
      setLoadingKeys(false);
      return;
    }
    setUser(current);
    setLoadingKeys(true);
    setLoadError('');
    try {
      const response = await portalApi.listKeys(current);
      if (!mountedRef.current) return;
      const activeKeys = (response.data || []).filter((key) => key.status === 'active');
      setApiKeys(activeKeys);
      const firstKeyId = activeKeys[0]?.id || '';
      setSelectedKeyId(firstKeyId);
      setLoadingKeys(false);
      if (firstKeyId) {
        await loadModelsForKey(current, firstKeyId);
      } else {
        setApiSecret('');
        setModels([]);
        setSelectedModel('');
      }
    } catch (error) {
      if (mountedRef.current) setLoadError(errorMessage(error));
    } finally {
      if (mountedRef.current) setLoadingKeys(false);
    }
  }, [loadModelsForKey]);

  useEffect(() => {
    mountedRef.current = true;
    const timer = window.setTimeout(() => void loadKeys(), 0);
    return () => {
      mountedRef.current = false;
      keyRequestRef.current += 1;
      window.clearTimeout(timer);
    };
  }, [loadKeys]);

  useEffect(() => {
    referenceImagesRef.current = referenceImages;
  }, [referenceImages]);

  useEffect(() => () => {
    referenceImagesRef.current.forEach((item) => URL.revokeObjectURL(item.previewUrl));
  }, []);

  useEffect(() => {
    const form = generationFormRef.current;
    if (!form) return;

    let frame = 0;
    const measure = () => {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(() => {
        const nextHeight = Math.ceil(form.getBoundingClientRect().height);
        setGenerationFormHeight((current) => current === nextHeight ? current : nextHeight);
      });
    };
    const observer = new ResizeObserver(measure);
    observer.observe(form);
    measure();
    return () => {
      observer.disconnect();
      window.cancelAnimationFrame(frame);
    };
  }, [apiKeys.length, loadingKeys]);

  const keyOptions = useMemo<AppSelectOption[]>(() => apiKeys.map((key) => ({
    value: key.id,
    label: `${key.name} · ${key.keyPrefix}`,
  })), [apiKeys]);

  const keySelectOptions = useMemo<AppSelectOption[]>(() => {
    if (keyOptions.length > 0) return keyOptions;
    return [{ value: '', label: loadingKeys ? '正在读取 Key...' : '暂无可用 Key', disabled: true }];
  }, [keyOptions, loadingKeys]);

  const modelOptions = useMemo<AppSelectOption[]>(() => {
    if (loadingModels) return [{ value: '', label: '正在读取模型...', disabled: true }];
    if (models.length === 0) return [{ value: '', label: '暂无可用模型', disabled: true }];
    return models.map((model) => ({ value: model.id, label: model.id }));
  }, [loadingModels, models]);

  const enabledSizeOptions = useMemo<AppSelectOption[]>(() => {
    const selected = models.find((model) => model.id === selectedModel);
    const enabled = sizeTiersForModel(selected);
    return sizeOptions.filter((option) => enabled.includes(option.value as SizeTier));
  }, [models, selectedModel]);

  const selectKey = (keyId: string) => {
    setSelectedKeyId(keyId);
    setGenerationError('');
    if (user && keyId) void loadModelsForKey(user, keyId);
  };

  const refreshModels = () => {
    if (user && selectedKeyId) void loadModelsForKey(user, selectedKeyId);
  };

  const selectModel = (modelId: string) => {
    setSelectedModel(modelId);
    const enabled = sizeTiersForModel(models.find((model) => model.id === modelId));
    setSizeTier((current) => enabled.includes(current) ? current : enabled[0]);
  };

  const addReferenceFiles = (files: File[]) => {
    if (generating || files.length === 0) return;
    const available = maxReferenceImages - referenceImages.length;
    if (available <= 0) {
      toast.error('最多上传 4 张参考图');
      return;
    }

    const validFiles = files.filter((file) => supportedReferenceImageTypes.has(file.type));
    if (validFiles.length !== files.length) toast.error('参考图仅支持 JPG、PNG、WEBP');
    const sizeValidFiles = validFiles.filter((file) => file.size <= maxReferenceImageBytes);
    if (sizeValidFiles.length !== validFiles.length) toast.error('单张参考图不能超过 20MB');
    if (sizeValidFiles.length > available) toast.error('最多上传 4 张参考图');

    let totalBytes = referenceImages.reduce((sum, item) => sum + item.file.size, 0);
    const accepted: File[] = [];
    for (const file of sizeValidFiles.slice(0, available)) {
      if (totalBytes + file.size > maxReferenceImagesTotalBytes) {
        toast.error('参考图总大小不能超过 75MB');
        break;
      }
      totalBytes += file.size;
      accepted.push(file);
    }
    if (accepted.length === 0) return;
    setReferenceImages((current) => [
      ...current,
      ...accepted.map((file) => ({
        id: `reference-${++referenceImageIDRef.current}`,
        file,
        previewUrl: URL.createObjectURL(file),
      })),
    ]);
  };

  const handleReferenceInput = (event: React.ChangeEvent<HTMLInputElement>) => {
    addReferenceFiles(Array.from(event.target.files || []));
    event.target.value = '';
  };

  const removeReferenceImage = (id: string) => {
    setReferenceImages((current) => {
      const removed = current.find((item) => item.id === id);
      if (removed) URL.revokeObjectURL(removed.previewUrl);
      return current.filter((item) => item.id !== id);
    });
  };

  const generate = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const cleanedPrompt = prompt.trim();
    if (!apiSecret || !selectedModel || !cleanedPrompt || generating) return;

    setGenerating(true);
    setGenerationError('');
    setResults([]);
    setCreatedAt(0);
    setLoadedImages(new Set<number>());
    setFailedImages(new Set<number>());
    try {
      const response = await portalApi.generateImages(apiSecret, {
        model: selectedModel,
        prompt: cleanedPrompt,
        n: quantity,
        size_tier: sizeTier,
        aspect_ratio: aspectRatio,
        output_format: outputFormat,
      }, referenceImages.map((item) => item.file));
      const urls = (response.data || []).map((item) => item.url).filter(Boolean);
      if (urls.length === 0) throw new Error('生成完成，但服务端没有返回图片');
      setResults(urls);
      setCreatedAt(response.created || Math.floor(Date.now() / 1000));
      if (user) {
        void refreshSession(user)
          .then((fresh) => { if (mountedRef.current) setUser(fresh); })
          .catch(() => undefined);
      }
      toast.success(`已生成 ${urls.length} 张图片`);
    } catch (error) {
      setGenerationError(errorMessage(error));
    } finally {
      setGenerating(false);
    }
  };

  const downloadImage = async (url: string, index: number) => {
    if (downloadingIndex !== null) return;
    setDownloadingIndex(index);
    try {
      const filename = `aipai-${Date.now()}-${index + 1}.${fileExtension(outputFormat)}`;
      const downloadURL = taskImageDownloadURL(url, filename);
      const anchor = document.createElement('a');
      anchor.href = downloadURL;
      anchor.download = filename;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      toast.success('已开始站内下载');
    } catch (error) {
      toast.error(errorMessage(error));
    } finally {
      setDownloadingIndex(null);
    }
  };

  const markImageLoaded = (index: number) => {
    setLoadedImages((current) => new Set(current).add(index));
  };

  const markImageFailed = (index: number) => {
    setFailedImages((current) => new Set(current).add(index));
  };

  const canGenerate = Boolean(apiSecret && selectedModel && prompt.trim() && !loadingModels && !generating);
  const ratioStyle = { aspectRatio: aspectRatio.replace(':', ' / ') };
  const generatedTime = formatGeneratedAt(createdAt);

  return (
    <div className="page-stack generation-page">
      <PageHeader title="生图台" description="文本与参考图生成 · OpenAI Images API">
        <Link href="/api-keys" className="btn"><KeyRound size={14} />管理 Key</Link>
        <button className="btn" type="button" onClick={() => void loadKeys()} disabled={loadingKeys || generating}>
          <RefreshCw size={14} className={loadingKeys ? 'animate-spin' : ''} />刷新
        </button>
      </PageHeader>

      {loadError && <div className="notice generation-error-notice" role="alert"><AlertCircle size={15} />{loadError}</div>}

      {!loadingKeys && apiKeys.length === 0 ? (
        <section className="section-panel generation-key-empty">
          {user?.emailVerifiedAt ? <KeyRound size={30} /> : <MailWarning size={30} />}
          <strong>{user?.emailVerifiedAt ? '还没有可用的 API Key' : '邮箱尚未验证'}</strong>
          <p>{user?.emailVerifiedAt ? '创建并启用 API Key 后即可进入生图台。' : '完成邮箱验证后，先创建 API Key 再开始生成。'}</p>
          <Link className="btn primary" href={user?.emailVerifiedAt ? '/api-keys' : '/settings'}>
            {user?.emailVerifiedAt ? <KeyRound size={14} /> : <MailWarning size={14} />}
            {user?.emailVerifiedAt ? '创建 API Key' : '验证邮箱'}
          </Link>
        </section>
      ) : (
        <div className="generation-workspace">
          <form ref={generationFormRef} className="section-panel generation-form" onSubmit={generate}>
            <div className="section-head generation-panel-head">
              <span><Sparkles size={15} /></span>
              <div><strong>生成参数</strong><small>{selectedModel || '模型准备中'}</small></div>
            </div>
            <div className="generation-form-body">
              <div className="field">
                <label htmlFor="generation-key">API Key</label>
                <AppSelect
                  id="generation-key"
                  value={selectedKeyId}
                  options={keySelectOptions}
                  onValueChange={selectKey}
                  disabled={loadingKeys || generating}
                />
              </div>

              <div className="field">
                <div className="generation-label-row">
                  <label htmlFor="generation-model">模型</label>
                  <button type="button" onClick={refreshModels} disabled={!selectedKeyId || loadingModels || generating} title="重新获取模型" aria-label="重新获取模型">
                    <RefreshCw size={12} className={loadingModels ? 'animate-spin' : ''} />
                  </button>
                </div>
                <AppSelect
                  id="generation-model"
                  value={selectedModel}
                  options={modelOptions}
                  onValueChange={selectModel}
                  disabled={loadingModels || models.length === 0 || generating}
                />
                {modelError && <span className="generation-field-error" role="alert"><AlertCircle size={12} />{modelError}</span>}
              </div>

              <div className="field generation-prompt-field">
                <div className="generation-label-row">
                  <label htmlFor="generation-prompt">提示词</label>
                  <span>{prompt.length}/4000</span>
                </div>
                <textarea
                  id="generation-prompt"
                  value={prompt}
                  onChange={(event) => setPrompt(event.target.value.slice(0, 4000))}
                  placeholder="例如：清晨薄雾中的山谷，远处有一座玻璃建筑，柔和自然光，电影感构图"
                  disabled={generating}
                  rows={7}
                />
              </div>

              <div className="field generation-reference-field">
                <div className="generation-label-row">
                  <label htmlFor="generation-reference-input">参考图</label>
                  <span>{referenceImages.length}/{maxReferenceImages}</span>
                </div>
                <input
                  ref={referenceInputRef}
                  id="generation-reference-input"
                  type="file"
                  accept="image/jpeg,image/png,image/webp"
                  multiple
                  hidden
                  disabled={generating || referenceImages.length >= maxReferenceImages}
                  onChange={handleReferenceInput}
                />
                <div
                  className={`generation-reference-tray ${referenceImages.length === 0 ? 'is-empty' : ''} ${referenceDragActive ? 'is-dragging' : ''}`}
                  onDragEnter={(event) => { event.preventDefault(); if (!generating) setReferenceDragActive(true); }}
                  onDragOver={(event) => event.preventDefault()}
                  onDragLeave={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setReferenceDragActive(false); }}
                  onDrop={(event) => {
                    event.preventDefault();
                    setReferenceDragActive(false);
                    addReferenceFiles(Array.from(event.dataTransfer.files || []));
                  }}
                >
                  {referenceImages.map((item, index) => (
                    <div className="generation-reference-item" key={item.id} title={item.file.name}>
                      <img src={item.previewUrl} alt={`参考图 ${index + 1}`} />
                      <span>{index + 1}</span>
                      <button type="button" onClick={() => removeReferenceImage(item.id)} disabled={generating} title={`删除参考图 ${index + 1}`} aria-label={`删除参考图 ${index + 1}`}>
                        <X size={12} />
                      </button>
                    </div>
                  ))}
                  {referenceImages.length < maxReferenceImages && (
                    <button className="generation-reference-upload" type="button" onClick={() => referenceInputRef.current?.click()} disabled={generating}>
                      <ImagePlus size={18} />
                      <span>{referenceImages.length === 0 ? '上传参考图' : '继续添加'}</span>
                    </button>
                  )}
                </div>
              </div>

              <div className="generation-parameter-grid">
                <div className="field">
                  <label htmlFor="generation-size">清晰度</label>
                  <AppSelect id="generation-size" value={sizeTier} options={enabledSizeOptions} onValueChange={(value) => setSizeTier(value as SizeTier)} disabled={generating || loadingModels} />
                </div>
                <div className="field">
                  <label htmlFor="generation-ratio">画面比例</label>
                  <AppSelect id="generation-ratio" value={aspectRatio} options={ratioOptions} onValueChange={(value) => setAspectRatio(value as AspectRatio)} disabled={generating} />
                </div>
                <div className="field">
                  <label htmlFor="generation-quantity">生成数量</label>
                  <AppSelect id="generation-quantity" value={String(quantity)} options={quantityOptions} onValueChange={(value) => setQuantity(Number(value))} disabled={generating} />
                </div>
                <div className="field">
                  <label htmlFor="generation-format">输出格式</label>
                  <AppSelect id="generation-format" value={outputFormat} options={formatOptions} onValueChange={(value) => setOutputFormat(value as OutputFormat)} disabled={generating} />
                </div>
              </div>

              <div className="generation-summary" aria-label="当前生成参数">
                <span>{sizeTier.toUpperCase()}</span>
                <span>{aspectRatio}</span>
                <span>{quantity} 张</span>
                <span>{outputFormat.toUpperCase()}</span>
                {referenceImages.length > 0 && <span>参考图 {referenceImages.length}</span>}
              </div>

              <button className="btn primary generation-submit" type="submit" disabled={!canGenerate}>
                {generating ? <LoaderCircle size={16} className="animate-spin" /> : <Sparkles size={16} />}
                {generating ? '生成中' : '开始生成'}
              </button>
            </div>
          </form>

          <section
            className="section-panel generation-results-panel"
            aria-live="polite"
            style={generationFormHeight ? { '--generation-form-height': `${generationFormHeight}px` } as React.CSSProperties : undefined}
          >
            <div className="section-head">
              <div>
                <strong>生成结果</strong>
                <small>{generating ? '任务处理中' : results.length > 0 ? `${results.length} 张 · ${generatedTime}` : '等待生成'}</small>
              </div>
              {results.length > 0 && !generating && (
                <button className="btn icon" type="button" onClick={() => setResults([])} title="清空结果" aria-label="清空结果">
                  <Trash2 size={14} />
                </button>
              )}
            </div>

            <div className={`generation-results-body ${results.length > 1 ? 'has-multiple' : ''}`}>
              {generating ? (
                <div className="generation-result-state">
                  <span className="generation-state-icon is-loading"><LoaderCircle size={26} className="animate-spin" /></span>
                  <strong>图片生成中</strong>
                  <small>{selectedModel} · {sizeTier.toUpperCase()} · {aspectRatio}{referenceImages.length > 0 ? ` · ${referenceImages.length} 张参考图` : ''}</small>
                </div>
              ) : generationError ? (
                <div className="generation-result-state is-error">
                  <span className="generation-state-icon"><AlertCircle size={26} /></span>
                  <strong>生成失败</strong>
                  <small>{generationError}</small>
                  <button className="btn" type="button" onClick={() => setGenerationError('')}>关闭</button>
                </div>
              ) : results.length === 0 ? (
                <div className="generation-result-state">
                  <span className="generation-state-icon"><Images size={27} /></span>
                  <strong>暂无生成结果</strong>
                  <small>{sizeTier.toUpperCase()} · {aspectRatio} · {outputFormat.toUpperCase()}</small>
                </div>
              ) : (
                <div className="generation-result-grid">
                  {results.map((url, index) => (
                    <article className="generation-result-item" key={`${url}-${index}`}>
                      <div className="generation-result-media" style={results.length > 1 ? ratioStyle : undefined}>
                        {!loadedImages.has(index) && !failedImages.has(index) && <LoaderCircle size={22} className="animate-spin generation-image-loader" />}
                        {failedImages.has(index) ? (
                          <div className="generation-image-error"><ImageIcon size={24} /><span>图片加载失败</span></div>
                        ) : (
                          <img
                            src={url}
                            alt={`生成结果 ${index + 1}：${prompt.trim()}`}
                            className={loadedImages.has(index) ? 'is-loaded' : ''}
                            onLoad={() => markImageLoaded(index)}
                            onError={() => markImageFailed(index)}
                          />
                        )}
                      </div>
                      <div className="generation-result-actions">
                        <span><ImageIcon size={13} />图片 {index + 1}</span>
                        <div>
                          <button className="btn icon" type="button" onClick={() => setPreviewImage(url)} title="查看大图" aria-label={`查看图片 ${index + 1}`}>
                            <Expand size={14} />
                          </button>
                          <button className="btn icon" type="button" onClick={() => void downloadImage(url, index)} disabled={downloadingIndex !== null} title="下载图片" aria-label={`下载图片 ${index + 1}`}>
                            {downloadingIndex === index ? <LoaderCircle size={14} className="animate-spin" /> : <Download size={14} />}
                          </button>
                        </div>
                      </div>
                    </article>
                  ))}
                </div>
              )}
            </div>
          </section>
        </div>
      )}

      {previewImage && (
        <div className="modal-backdrop generation-preview-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && setPreviewImage('')}>
          <div className="generation-preview-dialog" role="dialog" aria-modal="true" aria-label="图片预览">
            <button type="button" onClick={() => setPreviewImage('')} title="关闭预览" aria-label="关闭预览"><X size={18} /></button>
            <img src={previewImage} alt="生成图片预览" />
          </div>
        </div>
      )}
    </div>
  );
}
