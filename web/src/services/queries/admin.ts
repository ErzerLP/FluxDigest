import { useQuery } from '@tanstack/react-query';

import {
  getAdminConfigs,
  getAdminStatus,
  getJobRunDetail,
  getJobRuns,
} from '../api/admin';

export const adminQueryKeys = {
  status: ['admin', 'status'] as const,
  configs: ['admin', 'configs'] as const,
  jobs: (limit = 20) => ['admin', 'jobs', limit] as const,
  jobDetail: (jobId?: string) => ['admin', 'jobs', jobId ?? 'idle'] as const,
};

export function useAdminStatus() {
  return useQuery({
    queryKey: adminQueryKeys.status,
    queryFn: getAdminStatus,
  });
}

export function useAdminConfigs() {
  return useQuery({
    queryKey: adminQueryKeys.configs,
    queryFn: getAdminConfigs,
  });
}

export function useJobRuns(limit = 20) {
  return useQuery({
    queryKey: adminQueryKeys.jobs(limit),
    queryFn: () => getJobRuns(limit),
    retry: false,
  });
}

export function useJobRunDetail(jobId?: string) {
  return useQuery({
    queryKey: adminQueryKeys.jobDetail(jobId),
    queryFn: () => getJobRunDetail(jobId ?? ''),
    enabled: Boolean(jobId),
    retry: false,
  });
}
