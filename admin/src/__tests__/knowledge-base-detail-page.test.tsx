import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { KnowledgeBaseDetailPage } from '@/components/admin/knowledge-base-detail-page';

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));

vi.mock('@/services/auth/store', () => ({
  useAuth: () => ({
    user: { role: 'admin' },
  }),
}));

vi.mock('@/components/admin/knowledge-base-provider', () => ({
  useKnowledgeBaseContext: () => ({
    bases: [{ id: 7, name: 'bo-test', status: 'active', description: '' }],
    selectedBase: { id: 7, name: 'bo-test', status: 'active', description: '' },
    isLoading: false,
    error: null,
    isPermissionDenied: false,
    deleteBase: vi.fn(),
    setSelectedBaseId: vi.fn(),
  }),
}));

vi.mock('@/services/api/client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
  },
}));

import apiClient from '@/services/api/client';

const mockedGet = vi.mocked(apiClient.get);

describe('KnowledgeBaseDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedGet.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10 });
  });

  it('shows every file type accepted by the backend upload endpoint', async () => {
    render(<KnowledgeBaseDetailPage kbId={7} />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /上传文档/ })).toBeEnabled();
    });

    fireEvent.click(screen.getByRole('button', { name: /上传文档/ }));

    expect(
      screen.getByText('支持格式：PDF、MD、TXT、MARKDOWN、DOCX、HTML、HTM')
    ).toBeInTheDocument();
  });
});
