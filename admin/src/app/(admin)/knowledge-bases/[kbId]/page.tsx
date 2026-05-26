import { KnowledgeBaseDetailPage } from '@/components/admin/knowledge-base-detail-page';

export default function KnowledgeBaseDetailRoute({ params }: { params: { kbId: string } }) {
  return <KnowledgeBaseDetailPage kbId={Number(params.kbId)} />;
}
