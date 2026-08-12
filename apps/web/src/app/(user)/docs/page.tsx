'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { BookOpen, Check, Copy } from 'lucide-react';
import { toast } from 'sonner';
import { APIError, getSession, portalApi } from '@/lib/portal-api';
import styles from './page.module.css';

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
}> = {
  models: {
    label: '模型列表',
    method: 'GET',
    path: '/v1/models',
    description: '返回当前已启用、可通过 API 调用的模型。',
  },
  balance: {
    label: '余额查询',
    method: 'GET',
    path: '/v1/balance',
    description: '返回当前 API Key 所属账户的实时余额与该 Key 的计费模式。',
  },
  generations: {
    label: '图片生成',
    method: 'POST',
    path: '/v1/images/generations',
    description: '根据文本提示生成图片，支持多种模型、尺寸与输出格式。',
  },
  edits: {
    label: '图片编辑',
    method: 'POST',
    path: '/v1/images/edits',
    description: '上传参考图并根据提示词进行编辑，使用 multipart/form-data。',
  },
  chat: {
    label: '聊天兼容生图',
    method: 'POST',
    path: '/v1/chat/completions',
    description: '兼容 new-api 等默认通过 Chat Completions 测试图片模型的程序，实际任务仍按生图接口计费。',
  },
};

const requestParameters: Record<Endpoint, APIParameter[]> = {
  models: [],
  balance: [],
  generations: [
    { name: 'model', type: 'string', required: '是', description: '模型名称，参考模型列表接口获取' },
    { name: 'prompt', type: 'string', required: '是', description: '文本提示词' },
    { name: 'n', type: 'integer', required: '否', description: '生成图片数量，1-10' },
    { name: 'size', type: 'string', required: '否', description: '图片尺寸，如 1024x1024' },
    { name: 'quality', type: 'string', required: '否', description: '图片质量，standard 或 hd' },
    { name: 'aspect_ratio', type: 'string', required: '否', description: '宽高比，如 1:1、16:9、9:16' },
    { name: 'output_format', type: 'string', required: '否', description: '输出格式，url 或 b64_json' },
    { name: 'response_format', type: 'string', required: '否', description: '响应格式，url 或 b64_json' },
  ],
  edits: [
    { name: 'model', type: 'string', required: '是', description: '模型列表返回的 data[].id' },
    { name: 'prompt', type: 'string', required: '是', description: '图片编辑提示词' },
    { name: 'image / images', type: 'file / file[]', required: '二选一', description: 'multipart 参考图，最多 10 张，单张最大 20MB' },
    { name: 'image_url', type: 'string / string[]', required: '二选一', description: 'JSON 请求中的参考图 URL 或 data URI，最多 10 张' },
    { name: 'mask', type: 'file / string', required: '否', description: '可选遮罩图，支持 multipart 文件或 data URI' },
    { name: 'n', type: 'integer', required: '否', description: '输出数量，范围 1-10，默认 1' },
    { name: 'size', type: 'string', required: '否', description: '像素尺寸，例如 1024x1024；填写后优先使用' },
    { name: 'size_tier', type: 'string', required: '否', description: '清晰度：1k、2k、4k，默认 1k' },
    { name: 'quality', type: 'string', required: '否', description: '上游输出质量：auto、low、medium、high；仍兼容 1k、2k、4k 作为 size_tier 别名' },
    { name: 'resolution', type: 'string', required: '否', description: 'size_tier 的兼容别名' },
    { name: 'aspect_ratio / ratio', type: 'string', required: '否', description: '输出比例，例如 1:1、16:9、9:16' },
    { name: 'output_format', type: 'string', required: '否', description: 'png、jpeg、webp，默认 png' },
    { name: 'response_format', type: 'string', required: '否', description: 'url 或 b64_json，默认 url；b64_json 返回 Base64 图片数据' },
  ],
  chat: [
    { name: 'model', type: 'string', required: '是', description: '模型列表返回的图片模型 ID' },
    { name: 'messages', type: 'array', required: '是', description: '提取最后一条 user 消息作为真实生图提示词' },
    { name: 'stream', type: 'boolean', required: '否', description: '是否使用 Chat Completions SSE 流式响应，默认 false' },
    { name: 'n', type: 'integer', required: '否', description: '输出数量，范围 1-10，默认 1' },
    { name: 'size', type: 'string', required: '否', description: '像素尺寸，默认 1024x1024' },
    { name: 'quality', type: 'string', required: '否', description: '清晰度兼容参数' },
    { name: 'output_format', type: 'string', required: '否', description: 'png、jpeg、webp，默认 png' },
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
  "id": "img_1234567890",
  "object": "image.generation",
  "created": 1710000000,
  "data": [
    {
      "url": "https://cdn.example.com/images/img_1234567890.png"
    }
  ]
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
  const [endpoint, setEndpoint] = useState<Endpoint>('generations');
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
      curl: `curl -X POST '${baseUrl}/images/generations' \\
  -H 'Authorization: Bearer YOUR_API_KEY' \\
  -H 'Content-Type: application/json' \\
  -d '{
    "model": "MODEL_ID",
    "prompt": "产品摄影，白色背景，柔和棚拍光线",
    "n": 1,
    "size": "1024x1024",
    "response_format": "url"
  }'`,
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
  return (
    <div className={styles.page}>
      <section className={styles.authBar} aria-label="API 鉴权信息">
        <div className={styles.authorization}>
          <span className={styles.authLabel}>Authorization:</span>
          <code>Bearer YOUR_API_KEY</code>
        </div>
        <span className={styles.keyCount}>当前 {activeKeyCount} 个可用 Key</span>
        <div className={styles.baseUrlGroup}>
          <span className={styles.baseUrlLabel}>Base URL</span>
          <code className={styles.baseUrl}>{baseUrl}</code>
          <button
            className={styles.copyButton}
            type="button"
            onClick={() => void copy(baseUrl, 'base')}
            title="复制 Base URL"
            aria-label="复制 Base URL"
          >
            {copied === 'base' ? <Check size={17} /> : <Copy size={17} />}
          </button>
        </div>
        <Link href="/api-keys" className={styles.manageKey}>管理 Key</Link>
      </section>

      {loadError && <div className={styles.errorNotice} role="alert">{loadError}</div>}

      <section className={styles.documentation}>
        <nav className={styles.endpointNav} aria-label="API 接口">
          <div className={styles.navTitle}>
            <strong>接口</strong>
            <BookOpen size={16} aria-hidden="true" />
          </div>
          <div className={styles.endpointList}>
            {(Object.keys(endpointMeta) as Endpoint[]).map((item) => {
              const itemMeta = endpointMeta[item];
              return (
                <button
                  key={item}
                  type="button"
                  className={`${styles.endpointButton} ${endpoint === item ? styles.endpointActive : ''}`}
                  onClick={() => setEndpoint(item)}
                  aria-current={endpoint === item ? 'page' : undefined}
                >
                  <span className={`${styles.methodBadge} ${itemMeta.method === 'GET' ? styles.methodGet : styles.methodPost}`}>
                    {itemMeta.method}
                  </span>
                  <span className={styles.endpointCopy}>
                    <strong>{itemMeta.label}</strong>
                    <code>{itemMeta.path}</code>
                  </span>
                </button>
              );
            })}
          </div>
        </nav>

        <div className={styles.content}>
          <header className={styles.endpointHeader}>
            <div className={styles.endpointIdentity}>
              <span className={`${styles.methodBadge} ${meta.method === 'GET' ? styles.methodGet : styles.methodPost}`}>
                {meta.method}
              </span>
              <code>{meta.path}</code>
            </div>
            <p>{meta.description}</p>
          </header>

          <div className={styles.requestGrid}>
            <div className={styles.parameterColumn}>
              <section className={styles.tableSection} aria-label="请求参数">
                {requestParameters[endpoint].length > 0 ? (
                  <div className={styles.tableScroll}>
                    <table className={styles.table}>
                      <thead><tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr></thead>
                      <tbody>
                        {requestParameters[endpoint].map((parameter) => (
                          <tr key={parameter.name}>
                            <td><code>{parameter.name}</code></td>
                            <td><code>{parameter.type}</code></td>
                            <td>{parameter.required}</td>
                            <td>{parameter.description}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <div className={styles.emptyParameters}>无额外请求参数，仅需携带 Authorization 请求头。</div>
                )}
              </section>
            </div>

            <section className={styles.codePanel} aria-label="请求示例">
              <div className={styles.languageTabs}>
                {(['curl', 'javascript', 'python'] as Language[]).map((item) => (
                  <button
                    key={item}
                    type="button"
                    className={language === item ? styles.languageActive : ''}
                    onClick={() => setLanguage(item)}
                    aria-pressed={language === item}
                  >
                    {item === 'javascript' ? 'JavaScript' : item === 'python' ? 'Python' : 'cURL'}
                  </button>
                ))}
                <button
                  className={styles.codeCopy}
                  type="button"
                  onClick={() => void copy(samples[endpoint][language], 'sample')}
                  title="复制示例"
                  aria-label="复制示例"
                >
                  {copied === 'sample' ? <Check size={17} /> : <Copy size={17} />}
                </button>
              </div>
              <pre className={styles.codeBlock}><code>{samples[endpoint][language]}</code></pre>
            </section>
          </div>

          <section className={styles.responseGrid} aria-label="响应与错误">
            <div className={styles.responsePanel}>
              <div className={styles.sectionLabel}>
                <h2>成功响应示例（200）</h2>
              </div>
              <div className={styles.lightCodeWrap}>
                <button
                  className={styles.lightCopyButton}
                  type="button"
                  onClick={() => void copy(successResponses[endpoint], 'response')}
                  title="复制成功响应"
                  aria-label="复制成功响应"
                >
                  {copied === 'response' ? <Check size={16} /> : <Copy size={16} />}
                </button>
                <pre className={styles.lightCode}><code>{successResponses[endpoint].split('\n').map((line, index) => (
                  <span className={styles.codeLine} key={`${endpoint}-${index}`}>
                    <span className={styles.lineNumber}>{index + 1}</span>
                    <span>{line || ' '}</span>
                  </span>
                ))}</code></pre>
              </div>
            </div>

            <div className={styles.errorPanel}>
              <div className={styles.sectionLabel}><h2>错误码说明</h2></div>
              <div className={styles.tableScroll}>
                <table className={`${styles.table} ${styles.errorTable}`}>
                  <thead><tr><th>状态码</th><th>错误码</th><th>说明</th></tr></thead>
                  <tbody>
                    <tr><td><code>400</code></td><td><code>invalid_request</code></td><td>请求参数错误，请检查 model、prompt、n 与图片字段</td></tr>
                    <tr><td><code>401</code></td><td><code>unauthorized</code></td><td>API Key 缺失、失效或请求头不正确</td></tr>
                    <tr><td><code>402</code></td><td><code>insufficient_balance</code></td><td>余额或订阅额度不足</td></tr>
                    <tr><td><code>404</code></td><td><code>not_found</code></td><td>模型不存在或已停用</td></tr>
                    <tr><td><code>500 / 504</code></td><td><code>server_error</code></td><td>上游失败或请求超时，请稍后重试</td></tr>
                  </tbody>
                </table>
              </div>
            </div>
          </section>

        </div>
      </section>
    </div>
  );
}
