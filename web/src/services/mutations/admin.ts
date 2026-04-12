import { useMutation, useQueryClient } from '@tanstack/react-query';

import { runDailyDigest, testLLMConfig, updateLLMConfig } from '../api/admin';
import { adminQueryKeys } from '../queries/admin';

export function useSaveLLMConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: updateLLMConfig,
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: adminQueryKeys.configs }),
        queryClient.invalidateQueries({ queryKey: adminQueryKeys.status }),
      ]);
    },
  });
}

export function useTestLLMConfig() {
  return useMutation({
    mutationFn: testLLMConfig,
  });
}

export function useRunDailyDigest() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: runDailyDigest,
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: adminQueryKeys.status }),
        queryClient.invalidateQueries({ queryKey: adminQueryKeys.jobs() }),
        queryClient.invalidateQueries({ queryKey: adminQueryKeys.jobs(5) }),
        queryClient.invalidateQueries({ queryKey: adminQueryKeys.jobs(50) }),
      ]);
    },
  });
}
