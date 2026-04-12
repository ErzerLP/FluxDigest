type Tone = 'ok' | 'success' | 'warning' | 'error' | 'muted';

const statusMap: Record<string, { label: string; tone: Tone }> = {
  ok: { label: '正常', tone: 'ok' },
  succeeded: { label: '已完成', tone: 'success' },
  published: { label: '已发布', tone: 'success' },
  running: { label: '运行中', tone: 'warning' },
  pending: { label: '排队中', tone: 'warning' },
  unknown: { label: '未知', tone: 'muted' },
  failed: { label: '失败', tone: 'error' },
  error: { label: '异常', tone: 'error' },
  disabled: { label: '已禁用', tone: 'muted' },
  configured: { label: '已配置', tone: 'success' },
  missing: { label: '未配置', tone: 'warning' },
};

interface StatusBadgeProps {
  status?: string | null;
  fallbackLabel?: string;
}

export function StatusBadge({ status, fallbackLabel = '未知' }: StatusBadgeProps) {
  const normalized = status?.trim().toLowerCase() ?? 'unknown';
  const mapped = statusMap[normalized] ?? {
    label: status?.trim() || fallbackLabel,
    tone: 'muted' as const,
  };

  return (
    <span className={`status-badge status-badge-${mapped.tone}`}>
      <span className="status-badge-dot" />
      {mapped.label}
    </span>
  );
}
