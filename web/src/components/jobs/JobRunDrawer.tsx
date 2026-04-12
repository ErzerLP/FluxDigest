import { Drawer, Spin } from 'antd';

import type { JobRunDetail, JobRunRecord } from '../../types/admin';
import { StatusBadge } from '../status/StatusBadge';

interface JobRunDrawerProps {
  open: boolean;
  onClose: () => void;
  job?: JobRunRecord;
  detail?: JobRunDetail;
  loading?: boolean;
}

function renderValue(value?: string) {
  return value?.trim() ? value : '—';
}

export function JobRunDrawer({ open, onClose, job, detail, loading }: JobRunDrawerProps) {
  const detailPayload = detail?.detail ?? job?.detail;
  const remoteUrl =
    typeof detailPayload?.remote_url === 'string'
      ? detailPayload.remote_url
      : typeof detailPayload?.url === 'string'
        ? detailPayload.url
        : undefined;

  return (
    <Drawer
      open={open}
      onClose={onClose}
      width={520}
      title={job?.id ? `任务详情 · ${job.id}` : '任务详情'}
      className="job-drawer"
      getContainer={false}
      mask={false}
      destroyOnClose
    >
      {loading ? (
        <div className="drawer-loading">
          <Spin />
        </div>
      ) : (
        <div className="drawer-stack">
          <section className="detail-card">
            <div className="detail-grid">
              <div>
                <span className="detail-label">状态</span>
                <div className="detail-value">
                  <StatusBadge status={detail?.status ?? job?.status} />
                </div>
              </div>
              <div>
                <span className="detail-label">任务类型</span>
                <div className="detail-value monospace">
                  {renderValue(detail?.job_type ?? job?.job_type)}
                </div>
              </div>
              <div>
                <span className="detail-label">摘要日期</span>
                <div className="detail-value monospace">
                  {renderValue(detail?.digest_date ?? job?.digest_date)}
                </div>
              </div>
              <div>
                <span className="detail-label">触发来源</span>
                <div className="detail-value">
                  {renderValue(detail?.trigger_source ?? job?.trigger_source)}
                </div>
              </div>
            </div>
          </section>

          {remoteUrl ? (
            <section className="detail-card">
              <span className="detail-label">远端链接</span>
              <div className="detail-value monospace">{remoteUrl}</div>
            </section>
          ) : null}

          <section className="detail-card">
            <span className="detail-label">Detail JSON</span>
            <pre className="detail-json">{JSON.stringify(detailPayload ?? {}, null, 2)}</pre>
          </section>
        </div>
      )}
    </Drawer>
  );
}
