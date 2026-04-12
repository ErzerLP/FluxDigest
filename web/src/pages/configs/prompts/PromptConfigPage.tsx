import { PageHeader } from '../../../components/common/PageHeader';

export function PromptConfigPage() {
  return (
    <section className="page-stack">
      <div className="console-panel page-panel">
        <PageHeader
          eyebrow="Configuration"
          title="Prompts"
          subtitle="预留翻译、分析提示词模板的版本管理与验证入口。"
        />
        <div className="placeholder-shell-grid">
          <article className="data-card soft-card">
            <strong>模板分层</strong>
            <p>区分 system / task / publish prompts，后续可接版本对比与回滚。</p>
          </article>
          <article className="data-card soft-card">
            <strong>验证入口</strong>
            <p>后续接入 dry-run 与样例文章验证，减少提示词回归风险。</p>
          </article>
        </div>
      </div>
    </section>
  );
}
