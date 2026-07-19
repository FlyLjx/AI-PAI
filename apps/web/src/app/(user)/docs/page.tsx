'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import {
  AlertCircle,
  BookOpen,
  Check,
  Copy,
  FileImage,
  ImagePlus,
  KeyRound,
  ListTree,
  ShieldCheck,
  WalletCards,
} from 'lucide-react';
import { toast } from 'sonner';
import { PageHeader } from '@/components/common/PageHeader';
import { APIError, getSession, portalApi } from '@/lib/portal-api';

type Language = 'curl' | 'javascript' | 'python';
type Endpoint = 'models' | 'balance' | 'generations' | 'edits' | 'chat';
type APIParameter = {
  name: string;
  type: string;
  required: string;
  description: string;
};

const endpointMeta: Record<Endpoint, {
  label: string;
  method: 'GET' | 'POST';
  path: string;
  description: string;
  icon: typeof ListTree;
}> = {
  models: {
    label: '模型列表',
    method: 'GET',
    path: '/v1/models',
    description: '返回当前已启用、可通过 API 调用的模型。',
    icon: ListTree,
  },
  balance: {
    label: '余额查询',
    method: 'GET',
    path: '/v1/balance',
    description: '返回当前 API Key 所属账户的实时余额与该 Key 的计费模式。',
    icon: WalletCards,
  },
  generations: {
    label: '图片生成',
    method: 'POST',
    path: '/v1/images/generations',
    description: '使用文本提示词生成图片，响应格式兼容 OpenAI Images API。',
    icon: ImagePlus,
  },
  edits: {
    label: '图片编辑',
    method: 'POST',
    path: '/v1/images/edits',
    description: '上传参考图并根据提示词进行编辑，使用 multipart/form-data。',
    icon: FileImage,
  },
  chat: {
    label: '聊天兼容生图',
    method: 'POST',
    path: '/v1/chat/completions',
    description: '兼容 new-api 等默认通过 Chat Completions 测试图片模型的程序，实际任务仍按生图接口计费。',
    icon: ImagePlus,
  },
};

const requestParameters: Record<Endpoint, APIParameter[]> = {
  models: [],
  balance: [],
  generations: [
    { name: 'model', type: 'string', required: '是', description: '模型列表返回的 data[].id' },
    { name: 'prompt', type: 'string', required: '是', description: '图片生成提示词' },
    { name: 'n', type: 'integer', required: '否', description: '输出数量，范围 1-10，默认 1' },
    { name: 'size', type: 'string', required: '否', description: '像素尺寸，例如 1024x1024；填写后优先使用' },
    { name: 'size_tier', type: 'string', required: '否', description: '清晰度：1k、2k、4k，默认 1k' },
    { name: 'quality', type: 'string', required: '否', description: 'size_tier 的兼容别名' },
    { name: 'resolution', type: 'string', required: '否', description: 'size_tier 的兼容别名' },
    { name: 'aspect_ratio', type: 'string', required: '否', description: '比例：1:1、16:9、9:16、4:3、3:4、3:2、2:3' },
    { name: 'ratio', type: 'string', required: '否', description: 'aspect_ratio 的兼容别名' },
    { name: 'output_format', type: 'string', required: '否', description: 'jpeg、png、webp，默认 jpeg' },
    { name: 'background', type: 'string', required: '否', description: 'transparent 表示透明背景，并自动使用 PNG' },
    { name: 'referenceImages', type: 'string[]', required: '否', description: '参考图 URL 或 data URI 数组' },
    { name: 'response_format', type: 'string', required: '否', description: '当前使用 url，返回站内图片地址' },
  ],
  edits: [
    { name: 'model', type: 'string', required: '是', description: '模型列表返回的 data[].id' },
    { name: 'prompt', type: 'string', required: '是', description: '图片编辑提示词' },
    { name: 'image / images', type: 'file / file[]', required: '二选一', description: 'multipart 参考图，单张最大 20MB' },
    { name: 'image_url', type: 'string / string[]', required: '二选一', description: 'JSON 请求中的参考图 URL 或 data URI' },
    { name: 'mask', type: 'file / string', required: '否', description: '可选遮罩图，支持 multipart 文件或 data URI' },
    { name: 'n', type: 'integer', required: '否', description: '输出数量，范围 1-10，默认 1' },
    { name: 'size', type: 'string', required: '否', description: '像素尺寸，例如 1024x1024；填写后优先使用' },
    { name: 'size_tier', type: 'string', required: '否', description: '清晰度：1k、2k、4k，默认 1k' },
    { name: 'quality / resolution', type: 'string', required: '否', description: 'size_tier 的兼容别名' },
    { name: 'aspect_ratio / ratio', type: 'string', required: '否', description: '输出比例，例如 1:1、16:9、9:16' },
    { name: 'output_format', type: 'string', required: '否', description: 'jpeg、png、webp，默认 jpeg' },
    { name: 'background', type: 'string', required: '否', description: 'transparent 表示透明背景，并自动使用 PNG' },
    { name: 'response_format', type: 'string', required: '否', description: '当前使用 url，返回站内图片地址' },
  ],
  chat: [
    { name: 'model', type: 'string', required: '是', description: '模型列表返回的图片模型 ID' },
    { name: 'messages', type: 'array', required: '是', description: '提取最后一条 user 消息作为真实生图提示词' },
    { name: 'stream', type: 'boolean', required: '否', description: '是否使用 Chat Completions SSE 流式响应，默认 false' },
    { name: 'n', type: 'integer', required: '否', description: '输出数量，范围 1-10，默认 1' },
    { name: 'size', type: 'string', required: '否', description: '像素尺寸，默认 1024x1024' },
    { name: 'quality', type: 'string', required: '否', description: '清晰度兼容参数' },
    { name: 'output_format', type: 'string', required: '否', description: 'jpeg、png、webp，默认 jpeg' },
  ],
};

const responseParameters: Record<Endpoint, APIParameter[]> = {
  models: [
    { name: 'object', type: 'string', required: '-', description: '固定为 list' },
    { name: 'data[].id', type: 'string', required: '-', description: '调用图片接口时使用的模型 ID' },
    { name: 'data[].created', type: 'integer', required: '-', description: '模型创建时间，Unix 秒级时间戳' },
    { name: 'data[].owned_by', type: 'string', required: '-', description: '模型提供方' },
    { name: 'data[].enabled_size_tiers', type: 'string[]', required: '-', description: '模型已开放的 1k、2k、4k 清晰度' },
    { name: 'meta.total_count', type: 'integer', required: '-', description: '启用记录总数' },
    { name: 'meta.unique_count', type: 'integer', required: '-', description: '去重后的可调用模型数' },
  ],
  balance: [
    { name: 'object', type: 'string', required: '-', description: '固定为 balance' },
    { name: 'balance', type: 'number', required: '-', description: 'API Key 所属账户的实时余额' },
    { name: 'unit', type: 'string', required: '-', description: '余额单位，固定为 credits' },
    { name: 'billing_mode', type: 'string', required: '-', description: '当前 Key 的计费模式：balance 或 subscription' },
    { name: 'updated_at', type: 'string', required: '-', description: '账户余额最近更新时间，RFC3339 格式' },
  ],
  generations: [
    { name: 'created', type: 'integer', required: '-', description: '响应创建时间，Unix 秒级时间戳' },
    { name: 'data[].url', type: 'string', required: '-', description: '生成图片的站内下载地址' },
  ],
  edits: [
    { name: 'created', type: 'integer', required: '-', description: '响应创建时间，Unix 秒级时间戳' },
    { name: 'data[].url', type: 'string', required: '-', description: '编辑结果的站内下载地址' },
  ],
  chat: [
    { name: 'id', type: 'string', required: '-', description: 'Chat Completions 响应 ID' },
    { name: 'object', type: 'string', required: '-', description: '固定为 chat.completion' },
    { name: 'choices[0].message.content', type: 'string', required: '-', description: '包含站内图片地址的 Markdown 图片内容' },
    { name: 'choices[0].finish_reason', type: 'string', required: '-', description: '固定为 stop' },
    { name: 'usage', type: 'object', required: '-', description: '兼容 new-api 响应解析的用量对象' },
  ],
};

const successResponses: Record<Endpoint, string> = {
  models: `{
  "object": "list",
  "data": [{
    "id": "MODEL_ID",
    "object": "model",
    "enabled_size_tiers": ["1k", "2k"]
  }],
  "meta": { "total_count": 1, "unique_count": 1 }
}`,
  balance: `{
  "object": "balance",
  "balance": 128.75,
  "unit": "credits",
  "billing_mode": "balance",
  "updated_at": "2026-07-18T12:00:00+08:00"
}`,
  generations: `{
  "created": 1710000000,
  "data": [{ "url": "https://.../image.png" }]
}`,
  edits: `{
  "created": 1710000000,
  "data": [{ "url": "https://.../edited-image.png" }]
}`,
  chat: `{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1710000000,
  "model": "MODEL_ID",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "![image](https://.../image.png)"
    },
    "finish_reason": "stop"
  }],
  "usage": { "prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0 }
}`,
};

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : 'API Key 状态读取失败';
}

export default function DocsPage() {
  const [language, setLanguage] = useState<Language>('curl');
  const [endpoint, setEndpoint] = useState<Endpoint>('models');
  const [origin, setOrigin] = useState('https://YOUR_DOMAIN');
  const [activeKeyCount, setActiveKeyCount] = useState(0);
  const [loadError, setLoadError] = useState('');
  const [copied, setCopied] = useState('');

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setOrigin(window.location.origin);
      const current = getSession();
      if (!current) return;
      void portalApi.listKeys(current)
        .then((response) => setActiveKeyCount((response.data || []).filter((key) => key.status === 'active').length))
        .catch((error) => setLoadError(errorMessage(error)));
    }, 0);
    return () => window.clearTimeout(timer);
  }, []);

  const baseUrl = `${origin}/v1`;
  const samples = useMemo<Record<Endpoint, Record<Language, string>>>(() => ({
    models: {
      curl: `curl '${baseUrl}/models' -H 'Authorization: Bearer YOUR_API_KEY'`,
      javascript: `const response = await fetch('${baseUrl}/models', {
  headers: { Authorization: 'Bearer YOUR_API_KEY' },
});

if (!response.ok) throw new Error(await response.text());
const models = await response.json();
console.log(models.data);`,
      python: `import requests

response = requests.get(
    '${baseUrl}/models',
    headers={'Authorization': 'Bearer YOUR_API_KEY'},
    timeout=30,
)
response.raise_for_status()
print(response.json()['data'])`,
    },
    balance: {
      curl: `curl '${baseUrl}/balance' -H 'Authorization: Bearer YOUR_API_KEY'`,
      javascript: `const response = await fetch('${baseUrl}/balance', {
  headers: { Authorization: 'Bearer YOUR_API_KEY' },
});

const result = await response.json();
if (!response.ok) throw new Error(result.error?.message);
console.log(result.balance, result.billing_mode);`,
      python: `import requests

response = requests.get(
    '${baseUrl}/balance',
    headers={'Authorization': 'Bearer YOUR_API_KEY'},
    timeout=30,
)
response.raise_for_status()
result = response.json()
print(result['balance'], result['billing_mode'])`,
    },
    generations: {
      curl: `curl -X POST '${baseUrl}/images/generations' -H 'Authorization: Bearer YOUR_API_KEY' -H 'Content-Type: application/json' -d '{"model":"MODEL_ID","prompt":"产品摄影，白色背景，柔和棚拍光线","n":1,"size":"1024x1024","response_format":"url"}'`,
      javascript: `const response = await fetch('${baseUrl}/images/generations', {
  method: 'POST',
  headers: {
    Authorization: 'Bearer YOUR_API_KEY',
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    model: 'MODEL_ID',
    prompt: '产品摄影，白色背景，柔和棚拍光线',
    n: 1,
    size: '1024x1024',
    response_format: 'url',
  }),
});

const result = await response.json();
if (!response.ok) throw new Error(result.error?.message);
console.log(result.data.map((item) => item.url));`,
      python: `import requests

response = requests.post(
    '${baseUrl}/images/generations',
    headers={'Authorization': 'Bearer YOUR_API_KEY'},
    json={
        'model': 'MODEL_ID',
        'prompt': '产品摄影，白色背景，柔和棚拍光线',
        'n': 1,
        'size': '1024x1024',
        'response_format': 'url',
    },
    timeout=600,
)
response.raise_for_status()
print([item['url'] for item in response.json()['data']])`,
    },
    edits: {
      curl: `curl -X POST '${baseUrl}/images/edits' -H 'Authorization: Bearer YOUR_API_KEY' -F 'model=MODEL_ID' -F 'prompt=保留主体，将背景替换为简洁的摄影棚' -F 'image=@input.png' -F 'n=1' -F 'size=1024x1024'`,
      javascript: `const form = new FormData();
form.append('model', 'MODEL_ID');
form.append('prompt', '保留主体，将背景替换为简洁的摄影棚');
form.append('image', fileInput.files[0]);
form.append('n', '1');
form.append('size', '1024x1024');

const response = await fetch('${baseUrl}/images/edits', {
  method: 'POST',
  headers: { Authorization: 'Bearer YOUR_API_KEY' },
  body: form,
});

const result = await response.json();
if (!response.ok) throw new Error(result.error?.message);
console.log(result.data);`,
      python: `import requests

with open('input.png', 'rb') as image:
    response = requests.post(
        '${baseUrl}/images/edits',
        headers={'Authorization': 'Bearer YOUR_API_KEY'},
        data={
            'model': 'MODEL_ID',
            'prompt': '保留主体，将背景替换为简洁的摄影棚',
            'n': 1,
            'size': '1024x1024',
        },
        files={'image': ('input.png', image, 'image/png')},
        timeout=600,
    )
response.raise_for_status()
print(response.json()['data'])`,
    },
    chat: {
      curl: `curl -X POST '${baseUrl}/chat/completions' -H 'Authorization: Bearer YOUR_API_KEY' -H 'Content-Type: application/json' -d '{"model":"MODEL_ID","messages":[{"role":"user","content":"a cute cat"}],"stream":false}'`,
      javascript: `const response = await fetch('${baseUrl}/chat/completions', {
  method: 'POST',
  headers: {
    Authorization: 'Bearer YOUR_API_KEY',
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    model: 'MODEL_ID',
    messages: [{ role: 'user', content: 'a cute cat' }],
    stream: false,
  }),
});

const result = await response.json();
if (!response.ok) throw new Error(result.error?.message);
console.log(result.choices[0].message.content);`,
      python: `import requests

response = requests.post(
    '${baseUrl}/chat/completions',
    headers={'Authorization': 'Bearer YOUR_API_KEY'},
    json={
        'model': 'MODEL_ID',
        'messages': [{'role': 'user', 'content': 'a cute cat'}],
        'stream': False,
    },
    timeout=600,
)
response.raise_for_status()
print(response.json()['choices'][0]['message']['content'])`,
    },
  }), [baseUrl]);

  const copy = async (value: string, id: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(id);
      window.setTimeout(() => setCopied(''), 1600);
      toast.success('已复制');
    } catch {
      toast.error('复制失败，请手动选择内容');
    }
  };

  const meta = endpointMeta[endpoint];
  const MetaIcon = meta.icon;

  return (
    <div className="page-stack">
      <PageHeader title="API 文档" description="API Key 鉴权、余额与模型查询、图片生成、图片编辑及 new-api 兼容接口">
        <Link href="/api-keys" className="btn"><KeyRound size={14} />管理 Key</Link>
      </PageHeader>

      <section className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
        <div className="notice flex min-w-0 items-start gap-2">
          <ShieldCheck size={16} className="mt-0.5 shrink-0" />
          <span className="min-w-0">所有请求使用 <code className="mono break-all">Authorization: Bearer YOUR_API_KEY</code>。当前账户有 {activeKeyCount} 个可用 Key。</span>
        </div>
        <div className="section-panel flex min-w-0 items-center gap-2 px-3 py-2 text-[11px]">
          <span className="shrink-0 text-zinc-500">Base URL</span>
          <code className="mono min-w-0 flex-1 truncate">{baseUrl}</code>
          <button className="btn icon" type="button" onClick={() => void copy(baseUrl, 'base')} title="复制 Base URL" aria-label="复制 Base URL">
            {copied === 'base' ? <Check size={14} /> : <Copy size={14} />}
          </button>
        </div>
      </section>

      {loadError && <div className="notice" role="alert">{loadError}</div>}

      <section className="grid items-start gap-4 lg:grid-cols-[220px_minmax(0,1fr)]">
        <nav className="section-panel overflow-hidden" aria-label="API 接口">
          <div className="section-head"><strong>接口</strong><BookOpen size={15} className="text-zinc-400" /></div>
          <div className="grid gap-1 p-2">
            {(Object.keys(endpointMeta) as Endpoint[]).map((item) => {
              const itemMeta = endpointMeta[item];
              const Icon = itemMeta.icon;
              return (
                <button
                  key={item}
                  type="button"
                  className={`grid min-h-[48px] grid-cols-[24px_minmax(0,1fr)] items-center gap-2 rounded-md px-2 text-left ${endpoint === item ? 'bg-[#edf9f1] text-[#087443] shadow-[inset_3px_0_0_#16a35b]' : 'text-zinc-600 hover:bg-[#f5f7f5]'}`}
                  onClick={() => setEndpoint(item)}
                  aria-current={endpoint === item ? 'page' : undefined}
                >
                  <Icon size={15} />
                  <span className="min-w-0"><strong className="block text-[12px]">{itemMeta.label}</strong><code className="mt-0.5 block truncate text-[10px] opacity-70">{itemMeta.method} {itemMeta.path}</code></span>
                </button>
              );
            })}
          </div>
        </nav>

        <div className="grid gap-4">
          <article className="section-panel">
            <div className="section-head">
              <div className="flex min-w-0 items-center gap-2">
                <MetaIcon size={16} className="shrink-0 text-[#087443]" />
                <strong className="truncate">{meta.label}</strong>
              </div>
              <div className="flex items-center gap-2">
                <span className={`status-pill ${meta.method === 'GET' ? 'active' : 'processing'}`}>{meta.method}</span>
                <code className="mono hidden text-[11px] sm:block">{meta.path}</code>
              </div>
            </div>
            <div className="section-body">
              <p className="text-xs leading-5 text-zinc-600">{meta.description}</p>
              <div className="mt-4">
                <strong className="text-[12px]">请求参数</strong>
                {requestParameters[endpoint].length > 0 ? (
                  <div className="mt-2 data-table-wrap rounded-md border border-[#edf0ee]">
                    <table className="data-table">
                      <thead><tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr></thead>
                      <tbody>
                        {requestParameters[endpoint].map((parameter) => (
                          <tr key={parameter.name}>
                            <td className="mono whitespace-nowrap">{parameter.name}</td>
                            <td className="mono whitespace-nowrap">{parameter.type}</td>
                            <td>{parameter.required}</td>
                            <td>{parameter.description}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <div className="mt-2 rounded-md border border-[#edf0ee] bg-[#fafbf9] px-3 py-2.5 text-[11px] text-zinc-500">
                    无额外请求参数，仅需携带 Authorization 请求头。
                  </div>
                )}
              </div>
              <div className="mt-4">
                <strong className="text-[12px]">返回参数</strong>
                <div className="mt-4 data-table-wrap rounded-md border border-[#edf0ee]">
                  <table className="data-table">
                    <thead><tr><th>字段</th><th>类型</th><th>说明</th></tr></thead>
                    <tbody>
                      {responseParameters[endpoint].map((parameter) => (
                        <tr key={parameter.name}>
                          <td className="mono whitespace-nowrap">{parameter.name}</td>
                          <td className="mono whitespace-nowrap">{parameter.type}</td>
                          <td>{parameter.description}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          </article>

          <article className="section-panel overflow-hidden">
            <div className="section-head flex-wrap py-2">
              <strong>请求示例</strong>
              <div className="flex rounded-md border border-[#dce4df] bg-[#fafbf9] p-1">
                {(['curl', 'javascript', 'python'] as Language[]).map((item) => (
                  <button
                    key={item}
                    type="button"
                    className={`min-h-7 rounded px-2 text-[11px] font-bold ${language === item ? 'bg-white text-[#087443] shadow-sm' : 'text-zinc-500'}`}
                    onClick={() => setLanguage(item)}
                    aria-pressed={language === item}
                  >
                    {item === 'javascript' ? 'JavaScript' : item === 'python' ? 'Python' : 'cURL'}
                  </button>
                ))}
              </div>
            </div>
            <div className="relative bg-[#17201b]">
              <button
                className="absolute right-3 top-3 z-10 grid h-8 w-8 place-items-center rounded-md border border-white/15 bg-white/10 text-white hover:bg-white/20"
                type="button"
                onClick={() => void copy(samples[endpoint][language], 'sample')}
                title="复制示例"
                aria-label="复制示例"
              >
                {copied === 'sample' ? <Check size={14} /> : <Copy size={14} />}
              </button>
              <pre className="max-h-[430px] overflow-auto p-4 pr-14 text-[12px] leading-5 text-[#d8f5e4]"><code>{samples[endpoint][language]}</code></pre>
            </div>
          </article>

          <article className="section-panel">
            <div className="section-head"><strong>响应与错误</strong><AlertCircle size={15} className="text-zinc-400" /></div>
            <div className="section-body grid gap-4 xl:grid-cols-2">
              <div>
                <strong className="text-[12px]">成功响应</strong>
                <pre className="mt-2 overflow-auto rounded-md bg-[#f6f8f6] p-3 text-[11px] leading-5 text-zinc-700"><code>{successResponses[endpoint]}</code></pre>
              </div>
              <div>
                <strong className="text-[12px]">错误响应</strong>
                <pre className="mt-2 overflow-auto rounded-md bg-red-50 p-3 text-[11px] leading-5 text-red-800"><code>{`{
  "error": {
    "message": "错误说明",
    "type": "invalid_request_error",
    "param": null,
    "code": null
  }
}`}</code></pre>
              </div>
            </div>
            <div className="data-table-wrap border-t border-[#edf0ee]">
              <table className="data-table">
                <thead><tr><th>HTTP 状态</th><th>含义</th><th>处理</th></tr></thead>
                <tbody>
                  <tr><td className="mono">400</td><td>请求参数不正确</td><td>检查 model、prompt、n 与图片字段</td></tr>
                  <tr><td className="mono">401</td><td>API Key 缺失或失效</td><td>检查 Authorization 请求头及 Key 状态</td></tr>
                  <tr><td className="mono">402</td><td>余额或订阅额度不足</td><td>充值余额或购买订阅套餐</td></tr>
                  <tr><td className="mono">404</td><td>模型不存在或已停用</td><td>重新请求模型列表</td></tr>
                  <tr><td className="mono">500 / 504</td><td>上游失败或超时</td><td>记录请求信息后重试</td></tr>
                </tbody>
              </table>
            </div>
          </article>
        </div>
      </section>
    </div>
  );
}
