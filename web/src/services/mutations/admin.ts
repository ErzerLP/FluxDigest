import { useMutation, useQueryClient } from '@tanstack/react-query';

import {
  runDailyDigest,
  testLLMConfig,
  testMinifluxConfig,
  testPublishConfig,
  updateLLMConfig,
  updateMinifluxConfig,
  updatePromptConfig,
  updatePublishConfig,
} from '../api/admin';
import { adminQueryKeys } from '../queries/admin';

async function invalidateAdminConfigViews(queryClient: ReturnType<typeof useQueryClient>) {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: adminQueryKeys.configs }),
    queryClient.invalidateQueries({ queryKey: adminQueryKeys.status }),
  ]);
}

async function invalidateAdminStatusViews(queryClient: ReturnType<typeof useQueryClient>) {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: adminQueryKeys.status }),
    queryClient.invalidateQueries({ queryKey: adminQueryKeys.jobs() }),
    queryClient.invalidateQueries({ queryKey: adminQueryKeys.jobs(5) }),
    queryClient.invalidateQueries({ queryKey: adminQueryKeys.jobs(50) }),
  ]);
}

export function useSaveLLMConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: updateLLMConfig,
    onSuccess: async () => {
      await invalidateAdminConfigViews(queryClient);
    },
  });
}

export function useTestLLMConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: testLLMConfig,
    onSuccess: async () => {
      await invalidateAdminStatusViews(queryClient);
    },
  });
}

export function useSaveMinifluxConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: updateMinifluxConfig,
    onSuccess: async () => {
      await invalidateAdminConfigViews(queryClient);
    },
  });
}

export function useTestMinifluxConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: testMinifluxConfig,
    onSuccess: async () => {
      await invalidateAdminStatusViews(queryClient);
    },
  });
}

export function useSavePublishConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: updatePublishConfig,
    onSuccess: async () => {
      await invalidateAdminConfigViews(queryClient);
    },
  });
}

export function useTestPublishConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: testPublishConfig,
    onSuccess: async () => {
      await invalidateAdminStatusViews(queryClient);
    },
  });
}

export function useSavePromptConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: updatePromptConfig,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminQueryKeys.configs });
    },
  });
}

export function useRunDailyDigest() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: runDailyDigest,
    onSuccess: async () => {
      await invalidateAdminStatusViews(queryClient);
    },
  });
}
