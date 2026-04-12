import { PageHeader } from '../../../components/common/PageHeader';

export function PublishConfigPage() {
  return (
    <section className="page-stack">
      <div className="console-panel page-panel">
        <PageHeader
          eyebrow="Configuration"
          title="Publish"
          subtitle="预留发布目标、导出模式与渠道健康状态配置页。"
        />
        <div className="placeholder-shell-grid">
          <article className="data-card soft-card">
            <strong>发布目标</strong>
            <p>后续接入博客 API、静态导出或通知通道等多种 publish target。</p>
          </article>
          <article className="data-card soft-card">
            <strong>策略控件</strong>
            <p>计划支持发布开关、失败重试与 dry-run 对照输出。</p>
          </article>
        </div>
      </div>
    </section>
  );
}
