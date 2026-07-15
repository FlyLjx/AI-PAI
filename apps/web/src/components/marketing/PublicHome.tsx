'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import {
  Activity,
  ArrowRight,
  BookOpen,
  Check,
  CircleDollarSign,
  Code2,
  FileImage,
  Gauge,
  ImagePlus,
  KeyRound,
  ListTree,
  LockKeyhole,
  Network,
  ReceiptText,
  Route,
  ShieldCheck,
  WalletCards,
} from 'lucide-react';
import { getSession } from '@/lib/portal-api';
import { useRegistrationAvailability } from '@/lib/use-registration-availability';

type HomeDestination = {
  href: string;
  label: string;
};

const endpoints = [
  {
    method: 'GET',
    path: '/v1/models',
    title: '模型列表',
    description: '读取当前可调用的模型与标识。',
    icon: ListTree,
  },
  {
    method: 'POST',
    path: '/v1/images/generations',
    title: '图片生成',
    description: '提交文本提示词并获取图片结果。',
    icon: ImagePlus,
  },
  {
    method: 'POST',
    path: '/v1/images/edits',
    title: '图片编辑',
    description: '上传参考图并按提示词完成编辑。',
    icon: FileImage,
  },
];

const requestExample = `curl -X POST "$BASE_URL/v1/images/generations" \\
  -H "Authorization: Bearer $API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "MODEL_ID",
    "prompt": "产品摄影，白色背景",
    "n": 1,
    "size": "1024x1024"
  }'`;

function useHomeDestination() {
  const [destination, setDestination] = useState<HomeDestination>({
    href: '/register',
    label: '开始接入',
  });
  const [sessionReady, setSessionReady] = useState(false);

  useEffect(() => {
    const syncSession = () => {
      const user = getSession();
      if (user) {
        setDestination({ href: '/dashboard', label: '进入控制台' });
      } else {
        setDestination({ href: '/register', label: '开始接入' });
      }
      setSessionReady(true);
    };

    syncSession();
    window.addEventListener('storage', syncSession);
    window.addEventListener('aipai:session', syncSession);
    return () => {
      window.removeEventListener('storage', syncSession);
      window.removeEventListener('aipai:session', syncSession);
    };
  }, []);

  return { destination, sessionReady };
}

export function PublicHome() {
  const { destination: sessionDestination, sessionReady } = useHomeDestination();
  const registrationAvailability = useRegistrationAvailability();
  const isAuthenticated = sessionReady && sessionDestination.href === '/dashboard';
  const registrationOpen = registrationAvailability === 'open';
  const destination = isAuthenticated
    ? sessionDestination
    : registrationOpen
      ? { href: '/register', label: '开始接入' }
      : { href: '/login', label: '登录接入' };
  const billingHref = isAuthenticated ? '/billing' : destination.href;
  const docsHref = isAuthenticated ? '/docs' : destination.href;

  return (
    <div className="public-home">
      <header className="landing-nav">
        <div className="landing-container landing-nav-inner">
          <Link className="landing-brand" href="/" aria-label="AI-PAI 首页">
            <span className="brand-mark">AI</span>
            <span className="landing-brand-copy">
              <strong>AI-PAI</strong>
              <small>API 中转站</small>
            </span>
          </Link>

          <nav className="landing-links" aria-label="首页导航">
            <a href="#capabilities">API 能力</a>
            <a href="#billing">计费方式</a>
            <a href="#endpoints">接口</a>
            <a href="#access">接入流程</a>
          </nav>

          <div className="landing-auth">
            {isAuthenticated ? (
              <Link className="btn primary landing-nav-cta" href={destination.href}>
                {destination.label}
                <ArrowRight size={14} />
              </Link>
            ) : (
              <>
                <Link className="landing-login" href="/login">登录</Link>
                {registrationOpen && <Link className="btn primary landing-nav-cta" href="/register">注册</Link>}
              </>
            )}
          </div>
        </div>
      </header>

      <main>
        <section className="landing-hero" aria-labelledby="landing-title">
          <div className="landing-container landing-hero-grid">
            <div className="landing-hero-copy">
              <span className="landing-eyebrow"><Code2 size={15} /> OPENAI-COMPATIBLE IMAGE API</span>
              <h1 id="landing-title">统一接入图像模型，<br />只做稳定的 API 中转。</h1>
              <p>统一上游、鉴权、计费和日志，让团队专注业务集成。</p>
              <div className="landing-hero-actions">
                <Link className="btn primary landing-hero-cta" href={destination.href}>
                  {destination.label}
                  <ArrowRight size={15} />
                </Link>
                <a className="btn landing-hero-cta" href="#endpoints">
                  <BookOpen size={15} />
                  查看接口
                </a>
              </div>
            </div>

            <div className="landing-api-visual" role="region" aria-label="API 请求路径示意">
              <div className="landing-visual-head">
                <span><Route size={15} /> API 请求路径</span>
                <code>YOUR_DOMAIN/v1</code>
              </div>
              <div className="landing-route-flow">
                <div className="landing-route-node">
                  <Code2 size={19} />
                  <span><strong>你的服务</strong><small>OpenAI SDK / HTTP</small></span>
                </div>
                <ArrowRight className="landing-route-arrow" size={17} />
                <div className="landing-route-node is-primary">
                  <ShieldCheck size={19} />
                  <span><strong>AI-PAI</strong><small>鉴权、路由、计费</small></span>
                </div>
                <ArrowRight className="landing-route-arrow" size={17} />
                <div className="landing-route-node">
                  <Network size={19} />
                  <span><strong>上游模型</strong><small>统一响应</small></span>
                </div>
              </div>
              <div className="landing-request-preview">
                <div><span className="method post">POST</span><code>/images/generations</code></div>
                <pre><code>{`Authorization: Bearer YOUR_API_KEY\nContent-Type: application/json`}</code></pre>
              </div>
              <div className="landing-visual-foot">
                <span>OpenAI 兼容格式</span>
                <span>调用记录可追溯</span>
                <span>失败不扣费</span>
              </div>
            </div>
          </div>
        </section>

        <section className="landing-fact-band" aria-label="平台特点">
          <div className="landing-container landing-fact-grid">
            <div><KeyRound size={17} /><span><strong>独立 API Key</strong><small>按客户隔离凭证</small></span></div>
            <div><Route size={17} /><span><strong>统一模型入口</strong><small>减少上游适配成本</small></span></div>
            <div><Activity size={17} /><span><strong>调用全程留痕</strong><small>请求与结果可查询</small></span></div>
            <div><ReceiptText size={17} /><span><strong>双计费体系</strong><small>订阅额度或账户余额</small></span></div>
          </div>
        </section>

        <section className="landing-section" id="capabilities">
          <div className="landing-container">
            <div className="landing-section-heading">
              <h2>中转站需要的能力，集中在一个入口</h2>
              <p>前台只负责账户、凭证、文档和用量，不提供网页生图工作台。</p>
            </div>
            <div className="landing-capability-layout">
              <div className="landing-capability-list">
                <article>
                  <span><LockKeyhole size={18} /></span>
                  <div><h3>统一鉴权</h3><p>Bearer API Key 接入，支持创建、启停与轮换。</p></div>
                </article>
                <article>
                  <span><Network size={18} /></span>
                  <div><h3>上游路由</h3><p>模型与上游配置集中管理，客户端只维护一个入口。</p></div>
                </article>
                <article>
                  <span><Gauge size={18} /></span>
                  <div><h3>用量可观测</h3><p>按 Key 查看请求状态、模型、规格、图片数和时间。</p></div>
                </article>
                <article>
                  <span><ShieldCheck size={18} /></span>
                  <div><h3>运营可控制</h3><p>并发、模型、价格和客户状态统一由后台管理。</p></div>
                </article>
              </div>

              <aside className="landing-control-surface" aria-label="API 管理范围">
                <div className="landing-control-head">
                  <span>开发者工作台</span>
                  <code>API ONLY</code>
                </div>
                <div className="landing-control-row">
                  <KeyRound size={17} />
                  <span><strong>凭证管理</strong><small>创建、停用、并发控制</small></span>
                  <Check size={16} />
                </div>
                <div className="landing-control-row">
                  <Activity size={17} />
                  <span><strong>用量记录</strong><small>成功、失败与费用留痕</small></span>
                  <Check size={16} />
                </div>
                <div className="landing-control-row">
                  <WalletCards size={17} />
                  <span><strong>计费中心</strong><small>订阅额度与余额充值</small></span>
                  <Check size={16} />
                </div>
                <div className="landing-control-row">
                  <BookOpen size={17} />
                  <span><strong>接入文档</strong><small>cURL、JavaScript、Python</small></span>
                  <Check size={16} />
                </div>
              </aside>
            </div>
          </div>
        </section>

        <section className="landing-section landing-billing-section" id="billing">
          <div className="landing-container">
            <div className="landing-section-heading">
              <h2>订阅额度与余额扣费，按账户状态生效</h2>
              <p>有效付费订阅使用套餐额度；未开通付费订阅时按成功调用扣余额。</p>
            </div>
            <div className="landing-billing-grid">
              <article className="landing-billing-option is-subscription">
                <div className="landing-billing-title">
                  <span><WalletCards size={20} /></span>
                  <div><h3>订阅额度</h3><p>固定周期，固定图片额度</p></div>
                </div>
                <ul>
                  <li><Check size={15} />额度内调用不扣账户余额</li>
                  <li><Check size={15} />套餐可限定上游与模型范围</li>
                  <li><Check size={15} />适合调用量稳定的业务</li>
                </ul>
                <Link className="btn primary" href={billingHref}>查看订阅方式 <ArrowRight size={14} /></Link>
              </article>

              <article className="landing-billing-option">
                <div className="landing-billing-title">
                  <span><CircleDollarSign size={20} /></span>
                  <div><h3>余额扣费</h3><p>按模型单价，按成功调用扣费</p></div>
                </div>
                <ul>
                  <li><Check size={15} />没有付费订阅时使用账户余额</li>
                  <li><Check size={15} />按模型、规格和数量计算费用</li>
                  <li><Check size={15} />适合试接入与波动流量</li>
                </ul>
                <Link className="btn" href={billingHref}>查看余额计费 <ArrowRight size={14} /></Link>
              </article>
            </div>
          </div>
        </section>

        <section className="landing-section" id="endpoints">
          <div className="landing-container">
            <div className="landing-section-heading">
              <h2>保留三组 OpenAI 兼容接口</h2>
              <p>替换 Base URL 和 API Key，即可接入现有服务端调用链路。</p>
            </div>
            <div className="landing-endpoint-layout">
              <div className="landing-endpoint-list">
                {endpoints.map(({ method, path, title, description, icon: Icon }) => (
                  <article key={path}>
                    <span className={`method ${method === 'GET' ? 'get' : 'post'}`}>{method}</span>
                    <Icon size={18} />
                    <div><h3>{title}</h3><code>{path}</code><p>{description}</p></div>
                  </article>
                ))}
              </div>
              <div className="landing-code-sample">
                <div className="landing-code-head">
                  <span><Code2 size={15} /> cURL 请求示例</span>
                  <code>OpenAI Images</code>
                </div>
                <pre><code>{requestExample}</code></pre>
              </div>
            </div>
          </div>
        </section>

        <section className="landing-section landing-access-section" id="access">
          <div className="landing-container">
            <div className="landing-section-heading">
              <h2>从注册到首次调用，只保留必要步骤</h2>
              <p>不需要网页生图，所有图片请求都由你的服务端发起。</p>
            </div>
            <div className="landing-access-flow">
              <article>
                <span><KeyRound size={19} /></span>
                <h3>注册并创建 Key</h3>
                <p>进入开发者控制台，创建独立 API Key。</p>
              </article>
              <article>
                <span><Route size={19} /></span>
                <h3>替换接入地址</h3>
                <p>保留 OpenAI 请求格式，替换 Base URL 与凭证。</p>
              </article>
              <article>
                <span><Activity size={19} /></span>
                <h3>查看调用与成本</h3>
                <p>在控制台跟踪状态、额度、余额和历史记录。</p>
              </article>
            </div>
            <div className="landing-access-action">
              <Link className="btn primary" href={docsHref}>{isAuthenticated ? '打开 API 文档' : registrationOpen ? '注册后查看文档' : '登录后查看文档'} <ArrowRight size={14} /></Link>
            </div>
          </div>
        </section>

        <section className="landing-final-cta">
          <div className="landing-container landing-final-inner">
            <div>
              <h2>让你的产品直接对接图像 API</h2>
              <p>账户、Key、计费和日志都已集中到同一个开发者控制台。</p>
            </div>
            <Link className="btn primary landing-hero-cta" href={destination.href}>{destination.label} <ArrowRight size={15} /></Link>
          </div>
        </section>
      </main>

      <footer className="landing-footer">
        <div className="landing-container landing-footer-inner">
          <div className="landing-footer-brand">
            <span className="brand-mark">AI</span>
            <span><strong>AI-PAI</strong><small>面向服务端集成的图像 API 中转站</small></span>
          </div>
          <nav aria-label="页脚导航">
            <a href="#capabilities">API 能力</a>
            <a href="#billing">计费方式</a>
            {isAuthenticated ? (
              <Link href={destination.href}>{destination.label}</Link>
            ) : (
              <>
                <Link href="/login">登录</Link>
                {registrationOpen && <Link href="/register">注册</Link>}
              </>
            )}
          </nav>
        </div>
      </footer>
    </div>
  );
}
