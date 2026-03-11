'use client';

import { Typography, Button } from 'antd';
import type { FC } from 'react';

const { Title, Paragraph } = Typography;

const Banner: FC = () => {
  return (
    <div className="bg-gradient-to-r from-blue-50 to-purple-50 py-20 rounded-xl mb-8">
      <div className="container mx-auto px-4 text-center">
        <Title level={1} className="mb-4 font-bold">
          <span className="text-primary">AI面试</span>
          <span>大厂级面试特训平台</span>
        </Title>
        <Paragraph className="text-gray-600 text-lg mb-8 max-w-2xl mx-auto">
          押得准·定制简历押题 | 问得全·综合深度面试 | 评得细·垂直靶向训练
        </Paragraph>
        <div className="flex flex-col sm:flex-row justify-center gap-4">
          <Button type="primary" size="large" className="bg-primary hover:bg-primary/90">
            免费试用
          </Button>
          <Button size="large">了解更多</Button>
        </div>
        <div className="mt-8 text-gray-500 text-sm">
          <span className="mr-4">✓ 11,677+ 注册用户</span>
          <span className="mr-4">✓ 5,000+ AI 面试次数</span>
          <span>✓ 500+ 斩获Offer数量</span>
        </div>
      </div>
    </div>
  );
};

export default Banner;
