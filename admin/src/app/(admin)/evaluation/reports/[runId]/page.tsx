import { EvaluationReportPage } from '@/components/admin/evaluation-report-page';

type EvaluationReportRoutePageProps = {
  params: {
    runId: string;
  };
};

export default function EvaluationReportRoutePage({ params }: EvaluationReportRoutePageProps) {
  return <EvaluationReportPage runId={params.runId} />;
}
