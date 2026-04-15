import { useMemo, useState } from 'react';
import { Alert, Button, Spin } from 'antd';
import { useNavigate } from 'react-router-dom';

import { PageHeader } from '../../components/common/PageHeader';
import { StatusBadge } from '../../components/status/StatusBadge';
import { useRunDailyDigest } from '../../services/mutations/admin';
import { useAdminConfigs, useAdminStatus, useJobRuns } from '../../services/queries/admin';

function valueOrDash(value?: string) {
  return value?.trim() ? value : '—';
}

export function DashboardPage() {
  const navigate = useNavigate();
  const statusQuery = useAdminStatus();
  const configQuery = useAdminConfigs();
  const jobsQuery = useJobRuns(5);
  const runDailyDigestMutation = useRunDailyDigest();
  const [runMessage, setRunMessage] = useState<string>();
  const latestDigestStatus =
    statusQuery.data?.runtime?.latest_digest_status ?? statusQuery.data?.runtime?.latest_job_status;
  const latestJobs = jobsQuery.data?.items ?? [];
  const scheduler = configQuery.data?.scheduler;
  const publish = configQuery.data?.publish;

  const workflowSummary = useMemo(() => {
    const scheduleTime = scheduler?.schedule_time?.trim() || '07:00';
    const timezone = scheduler?.timezone?.trim() || 'Asia/Shanghai';
    return `${scheduleTime} / ${timezone}`;
  }, [scheduler?.schedule_time, scheduler?.timezone]);

  const articlePublishSummary = useMemo(() => {
    switch (publish?.article_publish_mode) {
      case 'all':
        return '全部单篇文章进入发布流程';
      case 'suggested':
        return '仅发布 AI 建议值得发送的文章';
      default:
        return '仅发布每日汇总日报';
    }
  }, [publish?.article_publish_mode]);

  const articleReviewSummary = useMemo(() => {
    switch (publish?.article_review_mode) {
      case 'auto_publish':
        return '自动发布';
      default:
        return '人工审核';
    }
  }, [publish?.article_review_mode]);

  const providerSummary = useMemo(() => {
    switch (publish?.provider) {
      case 'markdown_export':
        return 'Markdown 导出';
      case 'halo':
        return 'Halo 发布';
      default:
        return '未配置';
    }
  }, [publish?.provider]);

  async function handleRunDailyDigest() {
    try {
      const result = await runDailyDigestMutation.mutateAsync({ force: true });
      if (result.status === 'skipped') {
        setRunMessage('今日日报任务已存在，已跳过重复触发。');
        return;
      }

      setRunMessage('已受理今日日报生成任务。');
    } catch {
      setRunMessage(undefined);
    }
  }

  return (
    <section className="page-stack">
      <div className="console-panel page-panel">
        <PageHeader
          eyebrow="Overview"
          title="System Overview"
          subtitle="观察当前系统健康、集成状态与最近一次摘要运行结果。"
          actions={
            <Button
              type="primary"
              loading={runDailyDigestMutation.isPending}
              onClick={() => {
                void handleRunDailyDigest();
              }}
            >
              手动生成日报
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
        {runMessage ? <Alert type="success" showIcon message={runMessage} /> : null}
        {runDailyDigestMutation.isError ? (
          <Alert
            type="error"
            showIcon
            message="手动生成日报失败"
            description={
              runDailyDigestMutation.error instanceof Error
                ? runDailyDigestMutation.error.message
                : '未知错误'
            }
          />
        ) : null}

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

          <article className="data-card metric-card">
            <span className="metric-label">当前日报编排</span>
            <strong className="metric-value">
              {configQuery.isLoading ? <Spin size="small" /> : workflowSummary}
            </strong>
            <div className="metric-meta">
              <StatusBadge status={scheduler?.enabled === false ? 'disabled' : 'ok'} />
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
              <strong>发布设置</strong>
              <p>查看日报生成时间、单篇文章发布流程、审核策略与发布通道。</p>
              <Button
                type="default"
                onClick={() => {
                  navigate('/configs/publish');
                }}
              >
                去发布设置
              </Button>
            </article>
            <article className="data-card soft-card">
              <strong>模型健康检查</strong>
              <p>进入模型配置页检查 Base URL、API Key、模型名与连接状态。</p>
              <Button
                type="default"
                onClick={() => {
                  navigate('/configs/llm');
                }}
              >
                去模型配置
              </Button>
            </article>
            <article className="data-card soft-card">
              <strong>任务记录</strong>
              <p>查看最近日报、连通性测试与人工补跑记录，便于排查问题。</p>
              <Button
                type="default"
                onClick={() => {
                  navigate('/jobs');
                }}
              >
                查看任务记录
              </Button>
            </article>
          </div>
        </section>

        <section className="console-panel page-panel">
          <div className="section-heading-row">
            <div>
              <p className="section-eyebrow">Workflow summary</p>
              <h2 className="section-title">当前发布策略</h2>
            </div>
          </div>
          <div className="status-stack compact-stack">
            <div className="status-row">
              <span>自动日报</span>
              <span className="detail-value monospace">{workflowSummary}</span>
            </div>
            <div className="status-row">
              <span>发布方式</span>
              <span>{articlePublishSummary}</span>
            </div>
            <div className="status-row">
              <span>审核策略</span>
              <span>{articleReviewSummary}</span>
            </div>
            <div className="status-row">
              <span>输出通道</span>
              <span>{providerSummary}</span>
            </div>
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
