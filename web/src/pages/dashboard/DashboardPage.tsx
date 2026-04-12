import { Alert, Button, Spin } from 'antd';

import { PageHeader } from '../../components/common/PageHeader';
import { StatusBadge } from '../../components/status/StatusBadge';
import { useAdminStatus, useJobRuns } from '../../services/queries/admin';

function valueOrDash(value?: string) {
  return value?.trim() ? value : '—';
}

export function DashboardPage() {
  const statusQuery = useAdminStatus();
  const jobsQuery = useJobRuns(5);
  const latestDigestStatus =
    statusQuery.data?.runtime?.latest_digest_status ?? statusQuery.data?.runtime?.latest_job_status;

  const latestJobs = jobsQuery.data?.items ?? [];

  return (
    <section className="page-stack">
      <div className="console-panel page-panel">
        <PageHeader
          eyebrow="Overview"
          title="System Overview"
          subtitle="观察当前系统健康、集成状态与最近一次摘要运行结果。"
          actions={
            <Button type="primary" disabled>
              手动触发日报
            </Button>
          }
        />

        {statusQuery.isError ? (
          <Alert
            type="error"
            showIcon
            message="状态读取失败"
            description={statusQuery.error instanceof Error ? statusQuery.error.message : '未知错误'}
          />
        ) : null}
        <Alert type="info" showIcon message="Admin trigger 未接入，当前仅保留占位入口。" />

        <div className="dashboard-grid">
          <article className="data-card metric-card">
            <span className="metric-label">Latest digest</span>
            <strong className="metric-value monospace">
              {statusQuery.isLoading ? (
                <Spin size="small" />
              ) : (
                valueOrDash(statusQuery.data?.runtime?.latest_digest_date)
              )}
            </strong>
            <div className="metric-meta">
              <StatusBadge status={latestDigestStatus} />
            </div>
          </article>

          <article className="data-card metric-card">
            <span className="metric-label">LLM integration</span>
            <strong className="metric-value">
              {statusQuery.data?.integrations?.llm?.configured ? 'Connected' : 'Not configured'}
            </strong>
            <div className="metric-meta">
              <StatusBadge
                status={
                  statusQuery.data?.integrations?.llm?.configured
                    ? statusQuery.data?.integrations?.llm?.last_test_status ?? 'configured'
                    : 'missing'
                }
              />
            </div>
          </article>

          <article className="data-card metric-card">
            <span className="metric-label">System health</span>
            <div className="status-stack compact-stack">
              <div className="status-row">
                <span>API</span>
                <StatusBadge status={statusQuery.data?.system?.api} />
              </div>
              <div className="status-row">
                <span>DB</span>
                <StatusBadge status={statusQuery.data?.system?.db} />
              </div>
              <div className="status-row">
                <span>Redis</span>
                <StatusBadge status={statusQuery.data?.system?.redis} />
              </div>
            </div>
          </article>
        </div>
      </div>

      <div className="dashboard-columns">
        <section className="console-panel page-panel">
          <div className="section-heading-row">
            <div>
              <p className="section-eyebrow">Quick actions</p>
              <h2 className="section-title">操作入口</h2>
            </div>
          </div>
          <div className="action-card-grid">
            <article className="data-card soft-card">
              <strong>日报重跑</strong>
              <p>通过前端 future contract 直接触发 `daily-digest-run`，用于人工补跑。</p>
            </article>
            <article className="data-card soft-card">
              <strong>模型健康检查</strong>
              <p>转到 LLM Config 页面执行连接测试，确认代理与密钥仍然有效。</p>
            </article>
          </div>
        </section>

        <section className="console-panel page-panel">
          <div className="section-heading-row">
            <div>
              <p className="section-eyebrow">Recent jobs</p>
              <h2 className="section-title">最近任务摘要</h2>
            </div>
          </div>
          {jobsQuery.isError ? (
            <Alert
              type="error"
              showIcon
              message="最近任务读取失败"
              description={jobsQuery.error instanceof Error ? jobsQuery.error.message : '未知错误'}
            />
          ) : jobsQuery.isLoading ? (
            <div className="empty-state">
              <Spin />
            </div>
          ) : latestJobs.length ? (
            <div className="job-summary-list">
              {latestJobs.map((job) => (
                <article key={job.id} className="job-summary-item">
                  <div>
                    <div className="job-summary-title monospace">{valueOrDash(job.id)}</div>
                    <div className="job-summary-meta">
                      {valueOrDash(job.digest_date)} · {valueOrDash(job.job_type)}
                    </div>
                  </div>
                  <StatusBadge status={job.status} />
                </article>
              ))}
            </div>
          ) : (
            <div className="empty-state">当前没有任务记录。</div>
          )}
        </section>
      </div>
    </section>
  );
}
