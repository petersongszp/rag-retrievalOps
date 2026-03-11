'use client';

import { useState } from 'react';
import { Button, Card, Descriptions, Tag, message } from 'antd';
import { CheckCircleOutlined, CloseCircleOutlined, SyncOutlined } from '@ant-design/icons';
import { INTERVIEW_API } from '@/config/api';

interface HealthCheckResult {
  backendReachable: boolean;
  loginEndpoint: boolean;
  interviewEndpoint: boolean;
  corsSupport: boolean;
  error?: string;
}

export default function BackendHealthCheck() {
  const [checking, setChecking] = useState(false);
  const [result, setResult] = useState<HealthCheckResult | null>(null);

  const checkBackend = async () => {
    setChecking(true);
    const checkResult: HealthCheckResult = {
      backendReachable: false,
      loginEndpoint: false,
      interviewEndpoint: false,
      corsSupport: false,
    };

    try {
      // 1. 检查后端服务是否可达
      console.log('[诊断] 检查后端服务...');
      const baseResponse = await fetch(`${INTERVIEW_API.START_STREAM}`, {
        method: 'OPTIONS',
        mode: 'cors',
      });
      checkResult.backendReachable = true;
      checkResult.loginEndpoint = baseResponse.status < 500;

      // 2. 检查CORS支持
      const corsHeader = baseResponse.headers.get('Access-Control-Allow-Origin');
      checkResult.corsSupport = !!corsHeader;
      console.log('[诊断] CORS头:', corsHeader);

      // 3. 检查面试接口
      console.log('[诊断] 检查面试接口...');
      try {
        const token = localStorage.getItem('token');
        const interviewResponse = await fetch(`${INTERVIEW_API.START_STREAM}`, {
          method: 'OPTIONS',
          mode: 'cors',
          headers: token ? { Authorization: `Bearer ${token}` } : {},
        });
        checkResult.interviewEndpoint = interviewResponse.status !== 404;
        console.log('[诊断] 面试接口状态:', interviewResponse.status);
      } catch (e) {
        console.log('[诊断] 面试接口检查失败:', e);
        checkResult.interviewEndpoint = false;
      }

      setResult(checkResult);

      if (!checkResult.interviewEndpoint) {
        message.warning('面试接口返回404，可能需要检查后端路由配置');
      } else if (!checkResult.corsSupport) {
        message.warning('后端CORS配置可能有问题');
      } else {
        message.success('后端服务检查通过');
      }
    } catch (error: any) {
      console.error('[诊断] 检查失败:', error);
      checkResult.error = error.message || '网络错误';
      setResult(checkResult);
      const apiUrl = process.env.NEXT_PUBLIC_API_BASE_URL || '/api';
      message.error(`无法连接到后端服务，请确认后端是否运行在 ${apiUrl}`);
    } finally {
      setChecking(false);
    }
  };

  const StatusTag = ({ status }: { status: boolean }) => {
    return status ? (
      <Tag icon={<CheckCircleOutlined />} color="success">
        正常
      </Tag>
    ) : (
      <Tag icon={<CloseCircleOutlined />} color="error">
        异常
      </Tag>
    );
  };

  return (
    <Card
      title="后端服务诊断"
      extra={
        <Button type="primary" icon={<SyncOutlined />} loading={checking} onClick={checkBackend}>
          检查后端服务
        </Button>
      }
    >
      {!result && <p className="text-gray-500">点击"检查后端服务"按钮开始诊断</p>}

      {result && (
        <Descriptions column={1} bordered>
          <Descriptions.Item label="面试接口">
            <StatusTag status={result.interviewEndpoint} />
            {!result.interviewEndpoint && (
              <div className="mt-2 text-orange-600">
                <p>⚠️ 面试接口返回404，可能的原因：</p>
                <ul className="list-disc ml-6 mt-1">
                  <li>后端路由未正确注册</li>
                  <li>需要将 /api/mianshi/stream/start 添加到公共路由列表</li>
                  <li>JWT中间件拦截了请求</li>
                </ul>
              </div>
            )}
          </Descriptions.Item>

          <Descriptions.Item label="CORS支持">
            <StatusTag status={result.corsSupport} />
            {!result.corsSupport && (
              <span className="ml-2 text-orange-500">后端可能缺少CORS配置</span>
            )}
          </Descriptions.Item>

          {result.error && (
            <Descriptions.Item label="错误信息">
              <span className="text-red-500">{result.error}</span>
            </Descriptions.Item>
          )}
        </Descriptions>
      )}

      {result && !result.interviewEndpoint && (
        <div className="mt-4 p-4 bg-blue-50 rounded">
          <p className="font-bold mb-2">💡 解决方案：</p>
          <p>
            请在后端的 <code>middleware.go</code> 文件中，将以下路径添加到{' '}
            <code>jwtPublicRoutes</code>：
          </p>
          <pre className="mt-2 p-2 bg-white rounded border">
            {`"/api/mianshi/stream/start": {},
"/api/mianshi/answer/submit": {},
"/api/mianshi/interview/end": {},`}
          </pre>
        </div>
      )}
    </Card>
  );
}
