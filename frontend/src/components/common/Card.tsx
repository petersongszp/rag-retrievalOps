'use client';

import { Card as AntCard, CardProps as AntCardProps } from 'antd';

export interface CardProps extends AntCardProps {
  // 扩展自定义属性
}

const Card: React.FC<CardProps> = (props) => {
  return <AntCard {...props} />;
};

export default Card;
