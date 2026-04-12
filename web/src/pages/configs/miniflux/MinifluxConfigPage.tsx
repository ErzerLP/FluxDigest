import { PageHeader } from '../../../components/common/PageHeader';

export function MinifluxConfigPage() {
  return (
    <section className="page-stack">
      <div className="console-panel page-panel">
        <PageHeader
          eyebrow="Configuration"
          title="Miniflux"
          subtitle="预留 Miniflux 接入检查与同步窗口配置页。"
        />
        <div className="placeholder-shell-grid">
          <article className="data-card soft-card">
            <strong>接入状态</strong>
            <p>本轮仅提供结构化壳层，后续可接入 reader endpoint、token 与同步频率。</p>
          </article>
          <article className="data-card soft-card">
            <strong>待接入项</strong>
            <p>用户映射、分类过滤、轮询窗口与历史补抓策略。</p>
          </article>
        </div>
      </div>
    </section>
  );
}
