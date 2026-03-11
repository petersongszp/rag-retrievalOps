'use client';

import { useMemo, useState } from 'react';
import { Typography, Input, Select, Space, Button, Table, Tag, Empty, message, Card } from 'antd';
import {
  BookOutlined,
  SearchOutlined,
  DeleteOutlined,
  CopyOutlined,
  ReloadOutlined,
  TagsOutlined,
  FileTextOutlined,
} from '@ant-design/icons';

type Note = {
  key: number;
  type: '押题笔记' | '面试笔记';
  title: string;
  source: string;
  time: string;
  tags: string[];
};

const NOTES: Note[] = [
  {
    key: 1,
    type: '押题笔记',
    title: 'Redis持久化要点',
    source: '简历押题-Redis',
    time: '2024-11-02 12:40',
    tags: ['Redis', 'AOF'],
  },
  {
    key: 2,
    type: '面试笔记',
    title: 'Go并发最佳实践',
    source: '综合面试-社招',
    time: '2024-11-06 19:20',
    tags: ['Go', '并发'],
  },
  {
    key: 3,
    type: '押题笔记',
    title: 'MySQL索引设计',
    source: '简历押题-MySQL',
    time: '2024-11-03 21:05',
    tags: ['MySQL', '索引'],
  },
];

const ANSWERS: Record<number, string> = {
  1: '在Redis持久化方面，AOF用于记录写操作日志，RDB用于快照。结合两者可以在性能与可靠性之间取得平衡：AOF建议使用everysec策略保证数据安全，RDB用于低频全量备份。对于热点与大对象需谨慎持久化，避免阻塞与膨胀。',
  2: 'Go并发最佳实践包括合理使用goroutine与channel、避免共享可变状态、通过context控制生命周期、使用sync包（如WaitGroup、Mutex）做并发协调，并配合worker池与限流实现稳定的吞吐。',
  3: 'MySQL索引设计要点：优先考虑查询条件的选择性；前缀索引优化长字符串；合理使用覆盖索引减少回表；联合索引遵循最左前缀原则；避免在高频更新的低选择性字段上建立索引。',
};

export default function NotesPage() {
  const [active, setActive] = useState<'押题笔记' | '面试笔记'>('押题笔记');
  const [keyword, setKeyword] = useState('');
  const [tag, setTag] = useState<string | undefined>();
  const [notes, setNotes] = useState<Note[]>(NOTES);
  const [expandedKeys, setExpandedKeys] = useState<number[]>([]);

  const filtered = useMemo(() => {
    return notes
      .filter((n) => n.type === active)
      .filter((n) => (keyword ? n.title.includes(keyword) || n.source.includes(keyword) : true))
      .filter((n) => (tag ? n.tags.includes(tag) : true));
  }, [active, keyword, tag, notes]);

  const toggleExpand = (key: number) => {
    setExpandedKeys((prev) =>
      prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key]
    );
  };

  const handleDelete = (key: number) => {
    setNotes((prev) => prev.filter((n) => n.key !== key));
    message.success('笔记已删除');
  };

  const handleCopy = async (key: number) => {
    try {
      await navigator.clipboard.writeText(ANSWERS[key] || '');
      message.success('答案已复制到剪贴板');
    } catch {
      message.error('复制失败');
    }
  };

  return (
    <div className="min-h-screen relative font-sans">
      {/* Decorative Background */}
      <div className="fixed top-0 right-0 w-[600px] h-[600px] bg-emerald-50/60 rounded-full blur-[120px] -translate-y-1/2 translate-x-1/3 pointer-events-none z-0" />
      <div className="fixed bottom-0 left-0 w-[600px] h-[600px] bg-blue-50/60 rounded-full blur-[120px] translate-y-1/2 -translate-x-1/3 pointer-events-none z-0" />

      <div className="container mx-auto px-4 relative z-10">
        <div className="mb-8 animate-fade-in-up">
          <h1 className="text-3xl font-extrabold text-slate-900 tracking-tight flex items-center gap-3">
            <BookOutlined className="text-emerald-500" />
            笔记列表
          </h1>
          <p className="text-slate-500 mt-2 ml-11">整理你的面试知识库，温故而知新</p>
        </div>

        <div
          className="bg-white rounded-3xl p-8 border border-slate-100 shadow-xl shadow-slate-200/50 animate-fade-in-up"
          style={{ animationDelay: '0.1s' }}
        >
          {/* Tabs & Filters */}
          <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-8">
            <div className="bg-slate-100/80 p-1 rounded-xl inline-flex">
              {(['押题笔记', '面试笔记'] as const).map((type) => (
                <button
                  key={type}
                  onClick={() => setActive(type)}
                  className={`px-6 py-2 rounded-lg text-sm font-medium transition-all duration-300 ${
                    active === type
                      ? 'bg-white text-emerald-600 shadow-sm shadow-slate-200'
                      : 'text-slate-500 hover:text-slate-700'
                  }`}
                >
                  {type}
                </button>
              ))}
            </div>

            <div className="flex flex-wrap items-center gap-3">
              <Input
                placeholder="搜索关键词..."
                prefix={<SearchOutlined className="text-slate-400" />}
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                className="w-full md:w-[220px] h-10 rounded-lg border-slate-200 hover:border-emerald-400 focus:border-emerald-500"
                variant="filled"
              />
              <Select
                placeholder="选择标签"
                allowClear
                value={tag}
                onChange={setTag}
                className="w-full md:w-[160px] h-10"
                options={[
                  { value: 'Redis', label: 'Redis' },
                  { value: 'AOF', label: 'AOF' },
                  { value: 'Go', label: 'Go' },
                  { value: '并发', label: '并发' },
                  { value: 'MySQL', label: 'MySQL' },
                  { value: '索引', label: '索引' },
                ]}
                variant="filled"
              />
              <Button
                icon={<ReloadOutlined />}
                onClick={() => {
                  setKeyword('');
                  setTag(undefined);
                }}
                className="h-10 px-4 rounded-lg border-slate-200 text-slate-500 hover:text-emerald-600 hover:border-emerald-200"
              >
                重置
              </Button>
            </div>
          </div>

          {/* Content */}
          {filtered.length === 0 ? (
            <div className="py-20 text-center">
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={
                  <div className="text-slate-400">
                    <p className="mb-2">暂无{active}</p>
                    <p className="text-xs">尝试切换筛选条件或添加新笔记</p>
                  </div>
                }
              />
            </div>
          ) : (
            <div className="overflow-hidden rounded-xl border border-slate-100">
              <Table
                rowKey="key"
                expandable={{
                  expandedRowRender: (row: Note) => (
                    <div className="p-6 bg-slate-50/50 border-t border-slate-100">
                      <div className="bg-white rounded-xl p-6 border border-emerald-100 shadow-sm">
                        <div className="flex items-start gap-4">
                          <div className="bg-emerald-50 p-2 rounded-lg">
                            <FileTextOutlined className="text-emerald-600 text-xl" />
                          </div>
                          <div className="flex-1">
                            <div className="flex items-center gap-2 mb-3">
                              <h4 className="font-bold text-slate-800 m-0">参考答案与思路</h4>
                              <Tag
                                color="success"
                                className="rounded-full px-2 border-0 bg-emerald-50 text-emerald-600"
                              >
                                AI 生成
                              </Tag>
                            </div>
                            <div className="text-slate-600 leading-relaxed text-base">
                              {ANSWERS[row.key] || '暂无详细内容'}
                            </div>
                            <div className="mt-4 flex items-center gap-2">
                              <Button
                                size="small"
                                type="dashed"
                                icon={<CopyOutlined />}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  handleCopy(row.key);
                                }}
                                className="text-slate-500 hover:text-emerald-600 hover:border-emerald-300"
                              >
                                复制内容
                              </Button>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  ),
                  expandedRowKeys: expandedKeys,
                  onExpandedRowsChange: (keys) => setExpandedKeys(keys as number[]),
                  expandIcon: () => null, // Hide default icon, we use row click
                }}
                onRow={(row: Note) => ({
                  onClick: () => toggleExpand(row.key),
                  className: 'cursor-pointer hover:bg-slate-50 transition-colors group',
                })}
                pagination={{
                  pageSize: 10,
                  className: 'px-6 py-4',
                  showTotal: (total) => <span className="text-slate-400">共 {total} 条笔记</span>,
                }}
                dataSource={filtered}
                columns={[
                  {
                    title: '标题',
                    dataIndex: 'title',
                    className: 'pl-6',
                    render: (text) => (
                      <span className="font-bold text-slate-700 group-hover:text-emerald-700 transition-colors">
                        {text}
                      </span>
                    ),
                  },
                  {
                    title: '来源',
                    dataIndex: 'source',
                    render: (text) => (
                      <span className="text-slate-500 text-sm bg-slate-100 px-2 py-1 rounded-md">
                        {text}
                      </span>
                    ),
                  },
                  {
                    title: '标签',
                    dataIndex: 'tags',
                    render: (tags: string[]) => (
                      <div className="flex gap-1">
                        {tags.map((t) => (
                          <Tag
                            key={t}
                            bordered={false}
                            className="bg-blue-50 text-blue-600 m-0 rounded-full px-2.5"
                          >
                            {t}
                          </Tag>
                        ))}
                      </div>
                    ),
                  },
                  {
                    title: '创建时间',
                    dataIndex: 'time',
                    render: (text) => (
                      <span className="text-slate-400 text-xs font-mono">{text}</span>
                    ),
                  },
                  {
                    title: '操作',
                    width: 120,
                    render: (_: any, row: Note) => (
                      <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
                        <Button
                          type="text"
                          icon={<CopyOutlined />}
                          onClick={() => handleCopy(row.key)}
                          className="text-slate-400 hover:text-blue-600 hover:bg-blue-50"
                        />
                        <Button
                          type="text"
                          danger
                          icon={<DeleteOutlined />}
                          onClick={() => handleDelete(row.key)}
                          className="text-slate-400 hover:text-red-600 hover:bg-red-50"
                        />
                      </div>
                    ),
                  },
                ]}
                className="modern-table"
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
