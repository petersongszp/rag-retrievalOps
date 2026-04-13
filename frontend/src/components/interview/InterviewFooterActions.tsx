import { AudioOutlined, SendOutlined, StopOutlined } from '@ant-design/icons';
import { Button } from 'antd';

interface InterviewFooterActionsProps {
  isRecording: boolean;
  isStopping: boolean;
  speechDisabled: boolean;
  asrLoading: boolean;
  submitting: boolean;
  sendDisabled: boolean;
  onMicClick: () => void;
  onSubmit: () => void;
  sendButtonClassName: string;
  sendLabel?: string;
}

export function InterviewFooterActions({
  isRecording,
  isStopping,
  speechDisabled,
  asrLoading,
  submitting,
  sendDisabled,
  onMicClick,
  onSubmit,
  sendButtonClassName,
  sendLabel = 'Send',
}: InterviewFooterActionsProps) {
  return (
    <div className="relative z-10 flex justify-between items-center px-2 pb-2 pt-1 border-t border-slate-50">
      <div className="flex gap-1">
        {isRecording || isStopping ? (
          <Button
            danger
            shape="round"
            size="small"
            icon={<StopOutlined />}
            disabled={isStopping}
            onClick={onMicClick}
            className="!h-9 !px-4 !font-medium"
          >
            {isStopping ? 'Stopping...' : 'Stop Rec.'}
          </Button>
        ) : (
          <Button
            type="text"
            size="small"
            icon={<AudioOutlined className="text-slate-400" />}
            disabled={speechDisabled || asrLoading}
            onClick={onMicClick}
            className="!text-slate-400 !w-9 !h-9"
          />
        )}
      </div>
      <div className="flex items-center gap-3">
        <span className="text-xs text-slate-300 hidden sm:inline-block">Enter to Send</span>
        <Button
          type="primary"
          shape="round"
          icon={<SendOutlined />}
          loading={submitting}
          disabled={sendDisabled}
          onClick={onSubmit}
          className={sendButtonClassName}
        >
          {sendLabel}
        </Button>
      </div>
    </div>
  );
}
