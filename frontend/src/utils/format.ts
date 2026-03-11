// 格式化相关工具函数

/**
 * 格式化日期时间
 * @param date 日期字符串或时间戳
 */
export const formatDateTime = (date: string | number | Date): string => {
  // 预留实现
  return new Date(date).toLocaleString('zh-CN');
};

/**
 * 格式化数字
 * @param num 数字
 * @param decimals 小数位数
 */
export const formatNumber = (num: number, decimals: number = 0): string => {
  return num.toFixed(decimals);
};

/**
 * 格式化文件大小
 * @param bytes 字节数
 */
export const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 Bytes';

  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

/**
 * 截断文本
 * @param text 文本
 * @param maxLength 最大长度
 */
export const truncateText = (text: string, maxLength: number): string => {
  if (text.length <= maxLength) return text;
  return text.slice(0, maxLength) + '...';
};
