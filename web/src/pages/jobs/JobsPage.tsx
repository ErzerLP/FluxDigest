import { Alert, Button, Spin } from 'antd';
import { useMemo, useState } from 'react';

import { PageHeader } from '../../components/common/PageHeader';
import { JobRunDrawer } from '../../components/jobs/JobRunDrawer';
import { StatusBadge } from '../../components/status/StatusBadge';
import { useJobRunDetail, useJobRuns } from '../../services/queries/admin';

function displayText(value?: string) {
  return value?.trim() ? value : '—';
}

export function JobsPage() {
  const [selectedJobId, setSelectedJobId] = useState<string>();
  const jobsQuery = useJobRuns(50);
  const detailQuery = useJobRunDetail(selectedJobId);

  const selectedJob = useMemo(
    () => jobsQuery.data?.items?.find((item) => item.id === selectedJobId),
    [jobsQuery.data?.items, selectedJobId],
  );

  return (
    <section className="page-stack">
      <div className="console-panel page-panel">
        <PageHeader
          eyebrow="Operations"
          title="Jobs"
          subtitle="查看 job runs 并为后续人工触发与调试流程预留位置。"
        />

        {jobsQuery.isError ? (
          <Alert
            type="error"
            showIcon
            message="任务读取失败"
            description={jobsQuery.error instanceof Error ? jobsQuery.error.message : '未知错误'}
          />
        ) : null}

        {jobsQuery.isLoading ? (
          <div className="empty-state">
            <Spin />
          </div>
        ) : (
          <div className="job-list">
            {(jobsQuery.data?.items ?? []).map((job) => (
              <article key={job.id} className="job-row-card">
                <div className="job-row-main">
                  <div>
                    <div className="job-row-title monospace">{displayText(job.id)}</div>
                    <div className="job-row-subtitle">
                      {displayText(job.job_type)} · Digest {displayText(job.digest_date)}
                    </div>
                  </div>
                  <div className="job-row-status">
                    <StatusBadge status={job.status} />
                  </div>
                </div>
                <div className="job-row-actions">
                  <span className="job-row-muted">{displayText(job.trigger_source)}</span>
                  <Button onClick={() => setSelectedJobId(job.id)}>查看详情</Button>
                </div>
              </article>
            ))}
            {!jobsQuery.data?.items?.length ? <div className="empty-state">暂无任务运行记录。</div> : null}
          </div>
        )}
      </div>

      <JobRunDrawer
        open={Boolean(selectedJobId)}
        onClose={() => setSelectedJobId(undefined)}
        job={selectedJob}
        detail={detailQuery.data}
        loading={detailQuery.isLoading}
      />
    </section>
  );
}
