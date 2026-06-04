import { useState, useMemo, useCallback } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Search, RefreshCw, ExternalLink, Loader2, AlertTriangle,
  Database, X, LayoutGrid, LayoutList, ChevronDown, ChevronRight,
  ThumbsUp, MessageCircle, Share2, Eye, FileJson, FileSpreadsheet,
  ImageIcon, BadgeCheck, Flame, Hash, Clock,
} from 'lucide-react';
import { getResults, getResultStats, getSpiders, getStatsOverview, getTargetAccounts } from '../lib/api';
import type { ResultItem, Spider, StatsOverview } from '../types';

const PAGE_SIZE = 20;

// ==================== Helpers ====================

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return n.toLocaleString();
  return String(n);
}

function formatTime(iso: string): string {
  if (!iso) return '-';
  const d = new Date(iso);
  return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
}

function formatFullTime(iso: string): string {
  if (!iso) return '-';
  const d = new Date(iso);
  return d.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

/** Return the best display time: published_at if valid, else timestamp. */
function getDisplayTime(result: ResultItem): string {
  if (result.published_at) {
    const d = new Date(result.published_at);
    if (!isNaN(d.getTime()) && d.getTime() > 0) return result.published_at;
  }
  return result.timestamp;
}

function formatEngagement(n: number): string {
  if (n >= 10000) return `${(n / 10000).toFixed(1)}w`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
  return String(n);
}

function relativeTime(dateStr: string): string {
  if (!dateStr) return '';
  const now = Date.now();
  const d = new Date(dateStr).getTime();
  if (isNaN(d)) return dateStr;
  const diff = now - d;
  if (diff < 60_000) return '刚刚';
  if (diff < 3600_000) return `${Math.floor(diff / 60_000)}分钟前`;
  if (diff < 86400_000) return `${Math.floor(diff / 3600_000)}小时前`;
  if (diff < 2592000_000) return `${Math.floor(diff / 86400_000)}天前`;
  return formatTime(dateStr);
}

// ==================== Item field extraction ====================

function getNumField(item: Record<string, any>, keys: string[]): number | null {
  for (const k of keys) {
    if (item[k] !== undefined && item[k] !== null) {
      const n = Number(item[k]);
      if (!isNaN(n)) return n;
    }
  }
  return null;
}

/** Detect if this item is a topic summary (type=topic or legacy with link_content) */
function isTopic(item: Record<string, any>): boolean {
  return item.type === 'topic' || (item.link_content && typeof item.link_content === 'object' && !Array.isArray(item.link_content));
}

function getTitle(item: Record<string, any>): string {
  return item.title || item.topic_title || '';
}

function getText(item: Record<string, any>): string {
  return item.plain_text || item.full_text || item.text || item.content || item.description || item.summary || '';
}

function getAuthor(item: Record<string, any>): string {
  return item.user_name || item.screen_name || item.author || item.nickname || '';
}

function getAvatarUrl(item: Record<string, any>): string {
  return item.avatar_url || item.user_avatar || item.avatar || '';
}

function getImages(item: Record<string, any>): string[] {
  const candidates = item.images || item.local_images || item.image_urls;
  if (!Array.isArray(candidates)) return [];
  return candidates.map((u: any) => {
    if (typeof u === 'string' && u.length > 0) return u;
    if (u && typeof u === 'object' && typeof u.url === 'string') return u.url;
    return '';
  }).filter(Boolean);
}

function getComments(item: Record<string, any>): any[] {
  if (Array.isArray(item.comments)) return item.comments;
  return [];
}

function getNestedPosts(item: Record<string, any>): any[] {
  // Legacy support: old hot topic docs with embedded link_content.posts
  if (item.link_content && Array.isArray(item.link_content.posts)) return item.link_content.posts;
  return [];
}

function getSourceDevice(item: Record<string, any>): string {
  return item.source || '';
}

// ==================== Sub-components ====================

function Avatar({ url, name, size = 'md' }: { url: string; name: string; size?: 'sm' | 'md' | 'lg' }) {
  const dims = size === 'sm' ? 'w-7 h-7' : size === 'lg' ? 'w-12 h-12' : 'w-9 h-9';
  const textSize = size === 'sm' ? 'text-[10px]' : size === 'lg' ? 'text-base' : 'text-xs';
  const initial = name ? name.charAt(0).toUpperCase() : '?';

  if (url) {
    return (
      <img
        src={url}
        alt={name}
        className={`${dims} rounded-full object-cover bg-surface-container-high shrink-0`}
        onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; (e.target as HTMLImageElement).nextElementSibling && ((e.target as HTMLImageElement).nextElementSibling as HTMLElement).style.removeProperty('display'); }}
      />
    );
  }
  return (
    <div className={`${dims} rounded-full bg-primary/10 flex items-center justify-center ${textSize} font-bold text-primary shrink-0`}>
      {initial}
    </div>
  );
}

function ImageGrid({ images, onImageClick }: { images: string[]; onImageClick?: (idx: number) => void }) {
  if (images.length === 0) return null;
  const count = Math.min(images.length, 9);
  const gridCols = count === 1 ? 'grid-cols-1 max-w-sm' : count <= 4 ? 'grid-cols-2 max-w-md' : 'grid-cols-3 max-w-lg';

  return (
    <div className={`grid ${gridCols} gap-1.5 mt-2.5`}>
      {images.slice(0, 9).map((url, i) => (
        <div
          key={i}
          className="relative aspect-square rounded-lg overflow-hidden bg-surface-container-high cursor-pointer group"
          onClick={(e) => { e.stopPropagation(); onImageClick?.(i); }}
        >
          <img
            src={url}
            alt=""
            className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-200"
            loading="lazy"
            referrerPolicy="no-referrer"
          />
          {i === 8 && images.length > 9 && (
            <div className="absolute inset-0 bg-black/50 flex items-center justify-center">
              <span className="text-white text-sm font-bold">+{images.length - 9}</span>
            </div>
          )}
        </div>
      ))}
    </div>
  );
}

function EngagementBar({ item }: { item: Record<string, any> }) {
  const likes = getNumField(item, ['attitudes_count', 'likes', 'like_count', 'thumbs_up']);
  const comments = getNumField(item, ['comments_count', 'comment_count', 'comments']);
  const reposts = getNumField(item, ['reposts_count', 'repost_count', 'shares', 'share_count']);

  if (likes === null && comments === null && reposts === null) return null;

  return (
    <div className="flex gap-5 text-on-surface-variant">
      {likes !== null && (
        <div className="flex items-center gap-1.5">
          <ThumbsUp className="w-3.5 h-3.5" />
          <span className="text-xs">{formatEngagement(likes)}</span>
        </div>
      )}
      {comments !== null && (
        <div className="flex items-center gap-1.5">
          <MessageCircle className="w-3.5 h-3.5" />
          <span className="text-xs">{formatEngagement(comments)}</span>
        </div>
      )}
      {reposts !== null && (
        <div className="flex items-center gap-1.5">
          <Share2 className="w-3.5 h-3.5" />
          <span className="text-xs">{formatEngagement(reposts)}</span>
        </div>
      )}
    </div>
  );
}

function VerifiedBadge({ item }: { item: Record<string, any> }) {
  if (!item.verified) return null;
  const type = item.verified_type;
  const color = type === 0 ? 'text-amber-500' : type === 1 || type === 2 || type === 3 ? 'text-blue-500' : 'text-primary';
  return <BadgeCheck className={`w-3.5 h-3.5 ${color} shrink-0`} />;
}

// ==================== Post Card ====================

function PostCard({ result, onClick, avatarMap }: { result: ResultItem & { spider_name?: string }; onClick: () => void; avatarMap?: Record<string, string> }) {
  const item = result.item;
  const [expanded, setExpanded] = useState(false);

  const author = getAuthor(item);
  const avatar = getAvatarUrl(item) || (item.user_id && avatarMap?.[String(item.user_id)]) || '';
  const text = getText(item);
  const images = getImages(item);
  const comments = getComments(item);
  const source = getSourceDevice(item);
  const topicItem = isTopic(item);
  const nestedPosts = getNestedPosts(item);
  const summary = item.summary || item.link_content?.summary || '';
  const isLong = text.length > 280;
  const displayText = expanded ? text : (isLong ? text.slice(0, 280) + '...' : text);
  const keyword = item.keyword || '';
  const topicTitle = item.topic_title || '';

  // Topic summary cards: different layout
  if (topicItem && !author) {
    return (
      <div
        onClick={onClick}
        className="bg-surface-container-lowest rounded-xl hover:shadow-md transition-all cursor-pointer tonal-depth overflow-hidden"
      >
        <div className="p-5">
          <div className="flex justify-between items-start mb-2">
            <div className="flex items-center gap-2">
              <div className="w-9 h-9 rounded-lg bg-tertiary-container flex items-center justify-center shrink-0">
                <Flame className="w-4.5 h-4.5 text-on-tertiary-container" />
              </div>
              <div>
                <h3 className="text-[15px] font-bold text-on-surface leading-snug">
                  {item.rank && <span className="text-primary mr-1.5">#{item.rank}</span>}
                  {item.title}
                </h3>
                <p className="text-[10px] text-on-surface-variant">
                  {item.category && <span>{item.category}</span>}
                  {item.category && item.hot_value && <span className="mx-1 text-outline">·</span>}
                  {item.hot_value && <span className="text-tertiary">热度 {item.hot_value}</span>}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <span className="px-2 py-0.5 bg-tertiary-container text-on-tertiary-container text-[10px] font-bold rounded flex items-center gap-0.5">
                <Flame className="w-2.5 h-2.5" />热搜
              </span>
              {(result as any).spider_name && (
                <span className="px-2 py-0.5 bg-primary/10 text-primary text-[10px] font-bold rounded">
                  {(result as any).spider_name}
                </span>
              )}
            </div>
          </div>

          {summary && <p className="text-xs text-on-surface-variant leading-relaxed mt-2 mb-2">{summary}</p>}

          {item.post_count > 0 && (
            <p className="text-[10px] text-on-surface-variant">
              收录 <span className="text-primary font-bold">{item.post_count}</span> 条相关微博
            </p>
          )}

          {/* Legacy: nested posts from old data */}
          {nestedPosts.length > 0 && (
            <div className="mt-3 bg-surface-container-low rounded-lg p-3 space-y-2.5">
              <p className="text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">
                相关微博 ({nestedPosts.length})
              </p>
              {nestedPosts.slice(0, 3).map((post: any, i: number) => (
                <div key={post.id || i} className="flex gap-2.5">
                  <Avatar url={post.avatar_url || ''} name={post.user_name || ''} size="sm" />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1">
                      <span className="text-xs font-semibold text-on-surface truncate">{post.user_name || '匿名'}</span>
                      {post.verified && <BadgeCheck className="w-3 h-3 text-amber-500 shrink-0" />}
                    </div>
                    <p className="text-xs text-on-surface-variant line-clamp-2 leading-relaxed mt-0.5">{post.text || ''}</p>
                    <EngagementBar item={post} />
                  </div>
                </div>
              ))}
              {nestedPosts.length > 3 && (
                <p className="text-[10px] text-primary font-medium flex items-center gap-0.5">
                  查看全部 {nestedPosts.length} 条 <ChevronRight className="w-3 h-3" />
                </p>
              )}
            </div>
          )}

          <div className="flex items-center justify-between mt-3">
            <div className="flex items-center gap-2 text-[10px] text-outline">
              <span>{formatTime(getDisplayTime(result))}</span>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Regular post card (keyword search or hot topic individual post)
  return (
    <div
      onClick={onClick}
      className="bg-surface-container-lowest rounded-xl hover:shadow-md transition-all cursor-pointer tonal-depth overflow-hidden"
    >
      <div className="p-5">
        {/* Author header */}
        <div className="flex justify-between items-start mb-3">
          <div className="flex items-center gap-3 min-w-0">
            <Avatar url={avatar} name={author || (result as any).spider_name || '?'} />
            <div className="min-w-0">
              <div className="flex items-center gap-1.5">
                <h4 className="text-sm font-bold text-on-surface truncate">
                  {author || '未知用户'}
                </h4>
                <VerifiedBadge item={item} />
              </div>
              <p className="text-[10px] text-on-surface-variant flex items-center gap-1.5">
                <span>{relativeTime(getDisplayTime(result))}</span>
                {source && <><span className="text-outline">·</span><span>{source}</span></>}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            {topicTitle && (
              <span className="px-2 py-0.5 bg-tertiary-container text-on-tertiary-container text-[10px] font-medium rounded flex items-center gap-0.5">
                <Flame className="w-2.5 h-2.5" />{topicTitle.length > 12 ? topicTitle.slice(0, 12) + '...' : topicTitle}
              </span>
            )}
            {keyword && (
              <span className="px-2 py-0.5 bg-secondary-container text-on-secondary-container text-[10px] font-medium rounded">
                <Hash className="w-2.5 h-2.5 inline -mt-0.5 mr-0.5" />{keyword}
              </span>
            )}
            {(result as any).spider_name && (
              <span className="px-2 py-0.5 bg-primary/10 text-primary text-[10px] font-bold rounded">
                {(result as any).spider_name}
              </span>
            )}
          </div>
        </div>

        {/* Title (only for items with explicit title, not topic_title) */}
        {item.title && (
          <h3 className="text-[15px] font-bold text-on-surface mb-1.5 leading-snug">
            {item.rank && <span className="text-primary mr-1.5">#{item.rank}</span>}
            {item.title}
            {item.hot_value && (
              <span className="ml-2 text-xs font-normal text-tertiary">{item.hot_value}</span>
            )}
          </h3>
        )}

        {/* Post text content */}
        {displayText && (
          <div className="mb-2">
            <p className="text-sm text-on-surface leading-relaxed whitespace-pre-line">{displayText}</p>
            {isLong && (
              <button
                onClick={(e) => { e.stopPropagation(); setExpanded(!expanded); }}
                className="text-primary text-xs font-medium mt-1 hover:underline"
              >
                {expanded ? '收起' : '展开全文'}
              </button>
            )}
          </div>
        )}

        {/* Images */}
        <ImageGrid images={images} />

        {/* Comments preview */}
        {comments.length > 0 && (
          <div className="mt-3 bg-surface-container-low rounded-lg p-3 space-y-2">
            <p className="text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">
              热门评论 ({comments.length})
            </p>
            {comments.slice(0, 2).map((c: any, i: number) => (
              <div key={c.id || i} className="flex gap-2">
                <span className="text-xs font-semibold text-primary shrink-0">{c.user_name || '匿名'}</span>
                <span className="text-xs text-on-surface-variant line-clamp-1 flex-1">{c.text || ''}</span>
                {c.like_count > 0 && (
                  <span className="text-[10px] text-outline shrink-0 flex items-center gap-0.5">
                    <ThumbsUp className="w-2.5 h-2.5" />{c.like_count}
                  </span>
                )}
              </div>
            ))}
          </div>
        )}

        {/* Footer: engagement + meta */}
        <div className="flex items-center justify-between mt-3">
          <EngagementBar item={item} />
          <div className="flex items-center gap-2 text-[10px] text-outline">
            {item.category && <span>{item.category}</span>}
            <span>{formatTime(getDisplayTime(result))}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

// ==================== Detail Modal ====================

function DetailModal({ result, onClose, avatarMap }: { result: ResultItem & { spider_name?: string }; onClose: () => void; avatarMap?: Record<string, string> }) {
  const [lightboxIdx, setLightboxIdx] = useState<number | null>(null);
  const [showAllPosts, setShowAllPosts] = useState(false);
  const [showAllComments, setShowAllComments] = useState(false);
  const [showRawData, setShowRawData] = useState(false);

  const item = result.item;
  const author = getAuthor(item);
  const avatar = getAvatarUrl(item) || (item.user_id && avatarMap?.[String(item.user_id)]) || '';
  const title = getTitle(item);
  const text = getText(item);
  const images = getImages(item);
  const comments = getComments(item);
  const nestedPosts = getNestedPosts(item);
  const summary = item.summary || item.link_content?.summary || '';
  const source = getSourceDevice(item);
  const topicTitle = item.topic_title || '';
  const allImages = [...images];

  // Collect images from nested posts too
  nestedPosts.forEach((p: any) => {
    if (Array.isArray(p.images)) allImages.push(...p.images);
  });

  return (
    <>
      <div className="fixed inset-0 z-50 flex items-start justify-center pt-[5vh] bg-black/40" onClick={onClose}>
        <div className="bg-surface-container-lowest rounded-2xl w-[720px] max-h-[90vh] overflow-hidden tonal-depth" onClick={(e) => e.stopPropagation()}>
          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 bg-surface-container-low">
            <h2 className="text-sm font-bold text-on-surface flex items-center gap-2">
              <Database className="w-4 h-4 text-primary" />
              数据详情
            </h2>
            <button onClick={onClose} className="p-1 rounded-lg hover:bg-surface-container-highest transition-colors">
              <X className="w-4 h-4 text-on-surface-variant" />
            </button>
          </div>

          <div className="overflow-y-auto max-h-[calc(90vh-60px)] p-6 space-y-5">
            {/* Author info — only for posts that have an author */}
            {author ? (
              <div className="flex items-center gap-4">
                <Avatar url={avatar} name={author} size="lg" />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <h3 className="text-base font-bold text-on-surface">{author}</h3>
                    <VerifiedBadge item={item} />
                    {item.user_id && <span className="text-[10px] text-outline">ID: {item.user_id}</span>}
                  </div>
                  <div className="flex items-center gap-3 text-xs text-on-surface-variant mt-1">
                    <span className="flex items-center gap-1"><Clock className="w-3 h-3" />{formatFullTime(getDisplayTime(result))}</span>
                    {source && <span>{source}</span>}
                  </div>
                </div>
                {(result as any).spider_name && (
                  <span className="px-3 py-1 bg-primary/10 text-primary text-xs font-bold rounded-lg shrink-0">
                    {(result as any).spider_name}
                  </span>
                )}
              </div>
            ) : (
              <div className="flex items-center justify-between">
                <span className="flex items-center gap-1 text-xs text-on-surface-variant"><Clock className="w-3 h-3" />{formatFullTime(getDisplayTime(result))}</span>
                {(result as any).spider_name && (
                  <span className="px-3 py-1 bg-primary/10 text-primary text-xs font-bold rounded-lg shrink-0">
                    {(result as any).spider_name}
                  </span>
                )}
              </div>
            )}

            {/* Meta row */}
            <div className="flex flex-wrap gap-2">
              {topicTitle && (
                <span className="px-2.5 py-1 bg-tertiary-container text-on-tertiary-container text-xs rounded-lg flex items-center gap-1">
                  <Flame className="w-3 h-3" />热搜话题: {topicTitle}
                </span>
              )}
              {item.keyword && (
                <span className="px-2.5 py-1 bg-secondary-container text-on-secondary-container text-xs rounded-lg flex items-center gap-1">
                  <Hash className="w-3 h-3" />关键词: {item.keyword}
                </span>
              )}
              {item.rank && (
                <span className="px-2.5 py-1 bg-tertiary-container text-on-tertiary-container text-xs rounded-lg flex items-center gap-1">
                  <Flame className="w-3 h-3" />热搜 #{item.rank}
                </span>
              )}
              {item.hot_value && (
                <span className="px-2.5 py-1 bg-surface-container-high text-on-surface-variant text-xs rounded-lg">
                  热度: {item.hot_value}
                </span>
              )}
              {item.category && (
                <span className="px-2.5 py-1 bg-surface-container-high text-on-surface-variant text-xs rounded-lg">
                  {item.category}
                </span>
              )}
              {item.crawled_at && (
                <span className="px-2.5 py-1 bg-surface-container-high text-on-surface-variant text-xs rounded-lg">
                  采集: {item.crawled_at}
                </span>
              )}
            </div>

            {/* Title */}
            {title && (
              <h2 className="text-lg font-bold text-on-surface leading-snug">
                {item.rank && <span className="text-primary mr-1">#{item.rank}</span>}
                {title}
              </h2>
            )}

            {/* Summary */}
            {summary && (
              <div className="bg-surface-container-low rounded-lg p-4">
                <p className="text-sm text-on-surface-variant leading-relaxed">{summary}</p>
              </div>
            )}

            {/* Full content */}
            {item.html_content ? (
              <div className="bg-surface-container-low rounded-lg p-4">
                <div 
                  className="wechat-article-content"
                  dangerouslySetInnerHTML={{ __html: item.html_content }}
                />
              </div>
            ) : text && (
              <div>
                <p className="text-sm text-on-surface leading-relaxed whitespace-pre-line">{text}</p>
              </div>
            )}

            {/* Images - only show if no HTML content (images are embedded in HTML) */}
            {!item.html_content && images.length > 0 && (
              <div>
                <h4 className="text-xs font-bold text-on-surface-variant uppercase tracking-wider mb-2 flex items-center gap-1.5">
                  <ImageIcon className="w-3.5 h-3.5" />
                  图片 ({images.length})
                </h4>
                <ImageGrid images={images} onImageClick={(i) => setLightboxIdx(i)} />
              </div>
            )}

            {/* Engagement */}
            <div className="flex gap-6">
              <EngagementBar item={item} />
            </div>

            {/* URL — hide for topics (scheme is a mobile deep-link, not useful) */}
            {item.scheme && !isTopic(item) && (
              <a
                href={item.scheme}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 text-xs text-primary hover:underline"
              >
                查看原始链接 <ExternalLink className="w-3 h-3" />
              </a>
            )}

            {/* Nested posts (hot topics) */}
            {nestedPosts.length > 0 && (
              <div>
                <h4 className="text-xs font-bold text-on-surface-variant uppercase tracking-wider mb-3">
                  相关微博 ({nestedPosts.length})
                </h4>
                <div className="space-y-3">
                  {(showAllPosts ? nestedPosts : nestedPosts.slice(0, 5)).map((post: any, i: number) => (
                    <div key={post.id || i} className="bg-surface-container-low rounded-lg p-4 space-y-2">
                      <div className="flex items-center gap-2.5">
                        <Avatar url={post.avatar_url || ''} name={post.user_name || ''} size="sm" />
                        <div className="flex items-center gap-1.5 min-w-0 flex-1">
                          <span className="text-xs font-bold text-on-surface truncate">{post.user_name || '匿名'}</span>
                          {post.verified && <BadgeCheck className="w-3 h-3 text-amber-500 shrink-0" />}
                          <span className="text-[10px] text-outline ml-auto shrink-0">{post.created_at || ''}</span>
                        </div>
                      </div>
                      <p className="text-sm text-on-surface leading-relaxed whitespace-pre-line">
                        {post.full_text || post.text || ''}
                      </p>
                      {post.images && post.images.length > 0 && (
                        <div className="flex gap-1.5 flex-wrap">
                          {post.images.map((img: string, j: number) => (
                            <img key={j} src={img} alt="" className="w-16 h-16 rounded-lg object-cover bg-surface-container-high" loading="lazy" referrerPolicy="no-referrer" />
                          ))}
                        </div>
                      )}
                      <EngagementBar item={post} />
                    </div>
                  ))}
                </div>
                {nestedPosts.length > 5 && !showAllPosts && (
                  <button
                    onClick={() => setShowAllPosts(true)}
                    className="text-xs text-primary font-medium mt-2 hover:underline flex items-center gap-0.5"
                  >
                    查看全部 {nestedPosts.length} 条 <ChevronDown className="w-3 h-3" />
                  </button>
                )}
              </div>
            )}

            {/* Comments */}
            {comments.length > 0 && (
              <div>
                <h4 className="text-xs font-bold text-on-surface-variant uppercase tracking-wider mb-3">
                  评论 ({comments.length})
                </h4>
                <div className="space-y-2">
                  {(showAllComments ? comments : comments.slice(0, 5)).map((c: any, i: number) => (
                    <div key={c.id || i} className="flex gap-3 bg-surface-container-low rounded-lg p-3">
                      <span className="text-xs font-bold text-primary shrink-0">{c.user_name || '匿名'}</span>
                      <p className="text-xs text-on-surface flex-1 leading-relaxed">{c.text || ''}</p>
                      <div className="flex items-center gap-3 text-[10px] text-outline shrink-0">
                        {c.like_count > 0 && <span className="flex items-center gap-0.5"><ThumbsUp className="w-2.5 h-2.5" />{c.like_count}</span>}
                        {c.created_at && <span>{c.created_at}</span>}
                      </div>
                    </div>
                  ))}
                </div>
                {comments.length > 5 && !showAllComments && (
                  <button
                    onClick={() => setShowAllComments(true)}
                    className="text-xs text-primary font-medium mt-2 hover:underline flex items-center gap-0.5"
                  >
                    查看全部 {comments.length} 条评论 <ChevronDown className="w-3 h-3" />
                  </button>
                )}
              </div>
            )}

            {/* Raw data toggle */}
            <div>
              <button
                onClick={() => setShowRawData(!showRawData)}
                className="text-xs text-on-surface-variant font-medium flex items-center gap-1 hover:text-on-surface transition-colors"
              >
                <ChevronRight className={`w-3 h-3 transition-transform ${showRawData ? 'rotate-90' : ''}`} />
                原始数据字段
              </button>
              {showRawData && (
                <div className="mt-2 bg-surface-container-low rounded-lg p-4 space-y-1">
                  {Object.entries(item).map(([key, val]) => {
                    if (key === 'task_id') return null;
                    const display = typeof val === 'object' ? JSON.stringify(val, null, 2) : String(val ?? '');
                    const isLong = display.length > 300;
                    return (
                      <div key={key} className="flex gap-3 py-1.5">
                        <span className="text-[11px] text-on-surface-variant font-medium w-32 shrink-0 truncate font-mono" title={key}>{key}</span>
                        <span className={`text-xs text-on-surface ${isLong ? 'whitespace-pre-wrap break-all font-mono' : 'truncate'}`}>
                          {isLong ? display.slice(0, 500) + (display.length > 500 ? '...' : '') : display}
                        </span>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Image lightbox */}
      {lightboxIdx !== null && (
        <div className="fixed inset-0 z-[60] bg-black/80 flex items-center justify-center" onClick={() => setLightboxIdx(null)}>
          <button className="absolute top-4 right-4 p-2 bg-white/10 rounded-full hover:bg-white/20 transition-colors" onClick={() => setLightboxIdx(null)}>
            <X className="w-6 h-6 text-white" />
          </button>
          <img
            src={images[lightboxIdx]}
            alt=""
            className="max-w-[90vw] max-h-[90vh] object-contain rounded-lg"
            referrerPolicy="no-referrer"
            onClick={(e) => e.stopPropagation()}
          />
          {images.length > 1 && (
            <div className="absolute bottom-6 left-1/2 -translate-x-1/2 flex gap-2">
              {images.map((_, i) => (
                <button
                  key={i}
                  onClick={(e) => { e.stopPropagation(); setLightboxIdx(i); }}
                  className={`w-2 h-2 rounded-full transition-colors ${i === lightboxIdx ? 'bg-white' : 'bg-white/40'}`}
                />
              ))}
            </div>
          )}
        </div>
      )}
    </>
  );
}

// ==================== Main Component ====================

type ViewMode = 'table' | 'card';

export default function DataCenter() {
  const queryClient = useQueryClient();
  const [searchInput, setSearchInput] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [spiderFilter, setSpiderFilter] = useState('');
  const [page, setPage] = useState(1);
  const [viewMode, setViewMode] = useState<ViewMode>('card');
  const [selectedResult, setSelectedResult] = useState<(ResultItem & { spider_name?: string }) | null>(null);

  // Debounced search: commit query on Enter or blur
  const commitSearch = useCallback(() => {
    setSearchQuery(searchInput.trim());
    setPage(1);
  }, [searchInput]);

  // --- Queries ---
  const { data: statsData, isLoading: statsLoading } = useQuery<StatsOverview>({
    queryKey: ['stats-overview'],
    queryFn: () => getStatsOverview(),
  });

  const { data: resultStats, isLoading: resultStatsLoading } = useQuery({
    queryKey: ['result-stats'],
    queryFn: () => getResultStats(),
  });

  const {
    data: resultsData,
    isLoading: resultsLoading,
    error: resultsError,
  } = useQuery({
    queryKey: ['datacenter-results', page, spiderFilter, searchQuery],
    queryFn: () => getResults({
      page,
      size: PAGE_SIZE,
      ...(spiderFilter ? { spider_id: spiderFilter } : {}),
      ...(searchQuery ? { q: searchQuery } : {}),
    }),
  });

  const { data: spidersData } = useQuery({
    queryKey: ['datacenter-spiders'],
    queryFn: () => getSpiders({ page: 1, size: 100 }),
  });

  const { data: targetAccountsData } = useQuery({
    queryKey: ['datacenter-target-accounts'],
    queryFn: () => getTargetAccounts(),
    staleTime: 5 * 60 * 1000,
  });

  // --- Derived data ---
  const allResults: (ResultItem & { spider_name?: string })[] = (resultsData as any)?.data ?? [];
  const totalResults = (resultsData as any)?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(totalResults / PAGE_SIZE));
  const spiders: Spider[] = (spidersData as any)?.data ?? [];

  // Build avatar lookup from target accounts: user_id → avatar_url
  const targetAvatarMap = useMemo(() => {
    const map: Record<string, string> = {};
    if (targetAccountsData) {
      for (const a of targetAccountsData) {
        if (a.avatar_url) map[a.user_id] = a.avatar_url;
      }
    }
    return map;
  }, [targetAccountsData]);

  const spiderDistribution = useMemo(() => {
    const withResults = spiders
      .filter((s) => s.stat && s.stat.total_results > 0)
      .sort((a, b) => (b.stat?.total_results ?? 0) - (a.stat?.total_results ?? 0));
    const total = withResults.reduce((sum, s) => sum + (s.stat?.total_results ?? 0), 0);
    if (total === 0) return [];
    return withResults.slice(0, 8).map((s) => ({
      id: s._id,
      name: s.name,
      count: s.stat?.total_results ?? 0,
      pct: Math.round(((s.stat?.total_results ?? 0) / total) * 100),
    }));
  }, [spiders]);

  // --- Handlers ---
  const handleRefresh = () => {
    queryClient.invalidateQueries({ queryKey: ['datacenter-results'] });
    queryClient.invalidateQueries({ queryKey: ['result-stats'] });
    queryClient.invalidateQueries({ queryKey: ['stats-overview'] });
    queryClient.invalidateQueries({ queryKey: ['datacenter-spiders'] });
  };

  const handleExportJSON = () => {
    const data = allResults.map((r) => r.item);
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `data-export-${new Date().toISOString().slice(0, 10)}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleExportCSV = () => {
    if (allResults.length === 0) return;
    const allKeys = new Set<string>();
    allResults.forEach((r) => Object.keys(r.item).forEach((k) => allKeys.add(k)));
    const keys = Array.from(allKeys);
    const escape = (v: string) => `"${v.replace(/"/g, '""')}"`;
    const header = keys.map(escape).join(',');
    const rows = allResults.map((r) =>
      keys.map((k) => {
        const val = r.item[k];
        return escape(typeof val === 'object' ? JSON.stringify(val) : String(val ?? ''));
      }).join(',')
    );
    const csv = [header, ...rows].join('\n');
    const blob = new Blob(['\ufeff' + csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `data-export-${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  // --- Stats ---
  const statsCards = [
    {
      label: '总采集量',
      value: resultStatsLoading ? null : formatNumber((resultStats as any)?.total ?? 0),
      sub: '累计处理数据条数',
    },
    {
      label: '今日新增',
      value: resultStatsLoading ? null : formatNumber((resultStats as any)?.today_count ?? 0),
      sub: '今日采集数据量',
    },
    {
      label: '爬虫数量',
      value: statsLoading ? null : formatNumber(statsData?.spiders ?? 0),
      sub: '已注册爬虫总数',
      tag: statsData?.active_schedules ? `${statsData.active_schedules} 个定时` : '',
    },
    {
      label: '活跃节点数',
      value: statsLoading ? null : formatNumber(statsData?.nodes.online ?? 0),
      sub: '分布式集群状态',
      pulse: true,
      change: statsData ? `共 ${statsData.nodes.total} 个节点` : '',
    },
  ];

  return (
    <div className="flex gap-6">
      {/* Main Content */}
      <div className="flex-1 space-y-6 min-w-0">
        {/* Header */}
        <div className="flex items-end justify-between">
          <div>
            <h1 className="text-xl font-bold text-on-surface">数据中心</h1>
            <p className="text-sm text-on-surface-variant mt-1">检索、预览并导出多维分布式采集数据</p>
          </div>
          <div className="flex gap-2">
            <button
              onClick={handleExportCSV}
              className="flex items-center gap-1.5 px-3 py-2 text-xs font-medium text-on-surface-variant bg-surface-container-lowest rounded-lg hover:bg-surface-container-high transition-colors tonal-depth"
            >
              <FileSpreadsheet className="w-3.5 h-3.5" />导出 CSV
            </button>
            <button
              onClick={handleExportJSON}
              className="flex items-center gap-1.5 px-3 py-2 text-xs font-medium text-on-surface-variant bg-surface-container-lowest rounded-lg hover:bg-surface-container-high transition-colors tonal-depth"
            >
              <FileJson className="w-3.5 h-3.5" />JSON 导出
            </button>
          </div>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-4 gap-4">
          {statsCards.map((m) => (
            <div key={m.label} className="bg-surface-container-lowest rounded-xl p-4 tonal-depth">
              <div className="flex justify-between items-start">
                <p className="text-[11px] text-on-surface-variant">{m.label}</p>
                {'pulse' in m && m.pulse && (
                  <div className="flex items-center gap-1">
                    <span className="w-1.5 h-1.5 bg-primary rounded-full animate-pulse" />
                    <span className="text-[10px] text-primary font-medium">在线</span>
                  </div>
                )}
              </div>
              <div className="flex items-baseline gap-2 mt-1.5">
                <p className="text-xl font-bold text-on-surface">
                  {m.value === null ? (
                    <span className="inline-block w-12 h-5 bg-surface-container-high rounded animate-pulse" />
                  ) : m.value}
                </p>
                {'tag' in m && m.tag && <span className="text-[10px] font-medium text-primary">{m.tag}</span>}
                {'change' in m && m.change && <span className="text-[10px] text-primary">{m.change}</span>}
              </div>
              <p className="text-[10px] text-outline mt-1">{m.sub}</p>
            </div>
          ))}
        </div>

        {/* Filter Bar */}
        <div className="bg-surface-container-low rounded-xl p-4 space-y-3">
          <div className="grid grid-cols-12 gap-3">
            <div className="col-span-5 relative">
              <label className="block text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider mb-1">关键词搜索</label>
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-outline" />
                <input
                  type="text"
                  value={searchInput}
                  onChange={(e) => setSearchInput(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') commitSearch(); }}
                  onBlur={commitSearch}
                  placeholder="搜索内容、作者、关键词...  按 Enter 搜索"
                  className="w-full pl-9 pr-4 py-2 text-xs bg-surface-container-lowest rounded-lg outline-none focus:ring-2 focus:ring-primary/20 transition-all placeholder:text-outline-variant"
                />
              </div>
            </div>
            <div className="col-span-3">
              <label className="block text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider mb-1">爬虫来源</label>
              <div className="relative">
                <select
                  value={spiderFilter}
                  onChange={(e) => { setSpiderFilter(e.target.value); setPage(1); }}
                  className="w-full py-2 px-3 text-xs bg-surface-container-lowest rounded-lg outline-none focus:ring-2 focus:ring-primary/20 appearance-none cursor-pointer"
                >
                  <option value="">全部爬虫</option>
                  {spiders.map((s) => (
                    <option key={s._id} value={s._id}>{s.name}</option>
                  ))}
                </select>
                <ChevronDown className="absolute right-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-outline pointer-events-none" />
              </div>
            </div>
            <div className="col-span-3 flex items-end gap-2">
              <button
                onClick={handleRefresh}
                className="flex items-center gap-1 px-3 py-2 text-xs text-on-surface-variant bg-surface-container-lowest rounded-lg hover:bg-surface-container-high transition-colors"
              >
                <RefreshCw className="w-3 h-3" />刷新
              </button>
            </div>
            <div className="col-span-1 flex items-end justify-end gap-1">
              <button
                onClick={() => setViewMode('card')}
                className={`p-2 rounded-lg transition-colors ${viewMode === 'card' ? 'bg-primary text-on-primary' : 'text-on-surface-variant hover:bg-surface-container-high'}`}
                title="卡片视图"
              >
                <LayoutGrid className="w-3.5 h-3.5" />
              </button>
              <button
                onClick={() => setViewMode('table')}
                className={`p-2 rounded-lg transition-colors ${viewMode === 'table' ? 'bg-primary text-on-primary' : 'text-on-surface-variant hover:bg-surface-container-high'}`}
                title="表格视图"
              >
                <LayoutList className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>

        {/* Loading */}
        {resultsLoading && (
          <div className="bg-surface-container-lowest rounded-xl p-12 tonal-depth text-center">
            <Loader2 className="w-8 h-8 text-primary animate-spin mx-auto" />
            <p className="text-sm text-on-surface-variant mt-3">加载采集数据...</p>
          </div>
        )}

        {/* Error */}
        {resultsError && !resultsLoading && (
          <div className="bg-surface-container-lowest rounded-xl p-12 tonal-depth text-center">
            <AlertTriangle className="w-10 h-10 text-tertiary mx-auto mb-3" />
            <p className="text-sm font-medium text-on-surface">加载失败</p>
            <p className="text-xs text-on-surface-variant mt-1">{(resultsError as Error).message}</p>
            <button
              onClick={handleRefresh}
              className="mt-4 px-4 py-1.5 text-xs bg-primary text-on-primary rounded-lg hover:opacity-90 transition-opacity"
            >
              重试
            </button>
          </div>
        )}

        {/* Empty */}
        {!resultsLoading && !resultsError && allResults.length === 0 && (
          <div className="bg-surface-container-lowest rounded-xl p-12 tonal-depth text-center">
            <Database className="w-8 h-8 text-primary mx-auto mb-3" />
            <p className="text-sm font-medium text-on-surface">
              {searchQuery || spiderFilter ? '没有找到匹配的采集数据' : '暂无采集数据'}
            </p>
            <p className="text-xs text-on-surface-variant mt-1">
              {searchQuery || spiderFilter ? '请尝试调整搜索条件' : '运行爬虫后，采集的数据将显示在这里'}
            </p>
          </div>
        )}

        {/* Results Count */}
        {!resultsLoading && !resultsError && allResults.length > 0 && (
          <div className="flex items-center justify-between">
            <span className="text-sm text-on-surface-variant">
              找到 <span className="text-primary font-bold">{formatNumber(totalResults)}</span> 条数据
              {searchQuery && <span className="ml-1 text-on-surface-variant">· 搜索: "{searchQuery}"</span>}
            </span>
            <span className="text-xs text-on-surface-variant">
              第 {page} / {totalPages} 页
            </span>
          </div>
        )}

        {/* Card View */}
        {!resultsLoading && !resultsError && allResults.length > 0 && viewMode === 'card' && (
          <div className="space-y-3">
            {allResults.map((r) => (
              <PostCard key={r._id} result={r} onClick={() => setSelectedResult(r)} avatarMap={targetAvatarMap} />
            ))}
          </div>
        )}

        {/* Table View */}
        {!resultsLoading && !resultsError && allResults.length > 0 && viewMode === 'table' && (
          <div className="bg-surface-container-lowest rounded-xl tonal-depth overflow-hidden">
            <table className="w-full">
              <thead>
                <tr className="bg-surface-container-low">
                  <th className="text-left px-5 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">作者</th>
                  <th className="text-left px-5 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">内容摘要</th>
                  <th className="text-center px-3 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">互动</th>
                  <th className="text-left px-5 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">来源</th>
                  <th className="text-left px-5 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">发布时间</th>
                  <th className="text-right px-5 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-outline-variant/15">
                {allResults.map((r) => {
                  const author = getAuthor(r.item);
                  const avatar = getAvatarUrl(r.item) || (r.item.user_id && targetAvatarMap[String(r.item.user_id)]) || '';
                  const title = getTitle(r.item);
                  const text = getText(r.item);
                  const images = getImages(r.item);
                  const likes = getNumField(r.item, ['attitudes_count', 'likes', 'like_count']);
                  const commentsCount = getNumField(r.item, ['comments_count', 'comment_count']);
                  const reposts = getNumField(r.item, ['reposts_count', 'repost_count']);

                  return (
                    <tr key={r._id} className="hover:bg-surface-container-low transition-colors">
                      <td className="px-5 py-3">
                        <div className="flex items-center gap-2">
                          <Avatar url={avatar} name={author || '?'} size="sm" />
                          <div className="min-w-0">
                            <div className="flex items-center gap-1">
                              <span className="text-xs font-semibold text-on-surface truncate max-w-[100px]">{author || '-'}</span>
                              <VerifiedBadge item={r.item} />
                            </div>
                          </div>
                        </div>
                      </td>
                      <td className="px-5 py-3">
                        <div className="max-w-sm">
                          {title && <div className="text-xs font-bold text-on-surface truncate">{title}</div>}
                          <div className="text-xs text-on-surface-variant truncate">{text.slice(0, 100)}</div>
                          {images.length > 0 && (
                            <span className="inline-flex items-center gap-0.5 text-[10px] text-outline mt-0.5">
                              <ImageIcon className="w-2.5 h-2.5" />{images.length}张图片
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-3 py-3 text-center">
                        <div className="flex items-center justify-center gap-3 text-[10px] text-on-surface-variant">
                          {likes !== null && <span className="flex items-center gap-0.5"><ThumbsUp className="w-2.5 h-2.5" />{formatEngagement(likes)}</span>}
                          {commentsCount !== null && <span className="flex items-center gap-0.5"><MessageCircle className="w-2.5 h-2.5" />{formatEngagement(commentsCount)}</span>}
                          {reposts !== null && <span className="flex items-center gap-0.5"><Share2 className="w-2.5 h-2.5" />{formatEngagement(reposts)}</span>}
                        </div>
                      </td>
                      <td className="px-5 py-3">
                        <span className="text-xs text-on-surface-variant">{(r as any).spider_name || '-'}</span>
                      </td>
                      <td className="px-5 py-3 text-xs text-on-surface-variant font-mono">{formatTime(r.timestamp)}</td>
                      <td className="px-5 py-3 text-right">
                        <button
                          onClick={() => setSelectedResult(r)}
                          className="text-xs text-primary hover:underline flex items-center gap-1 ml-auto"
                        >
                          详情 <Eye className="w-3 h-3" />
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {!resultsLoading && !resultsError && totalPages > 1 && (
          <div className="flex items-center justify-center gap-1 pt-2">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page <= 1}
              className="px-3 py-1.5 text-xs rounded-lg bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
            >
              上一页
            </button>
            {Array.from({ length: Math.min(totalPages, 7) }, (_, i) => {
              let pageNum: number;
              if (totalPages <= 7) {
                pageNum = i + 1;
              } else if (page <= 4) {
                pageNum = i + 1;
              } else if (page >= totalPages - 3) {
                pageNum = totalPages - 6 + i;
              } else {
                pageNum = page - 3 + i;
              }
              return (
                <button
                  key={pageNum}
                  onClick={() => setPage(pageNum)}
                  className={`w-7 h-7 text-xs rounded-lg transition-colors ${
                    page === pageNum
                      ? 'bg-primary text-on-primary font-medium'
                      : 'bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest'
                  }`}
                >
                  {pageNum}
                </button>
              );
            })}
            <button
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
              className="px-3 py-1.5 text-xs rounded-lg bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
            >
              下一页
            </button>
          </div>
        )}
      </div>

      {/* Right Sidebar */}
      <div className="w-72 shrink-0 space-y-4">
        <div className="bg-surface-container-lowest rounded-xl p-5 tonal-depth">
          <h3 className="text-sm font-semibold text-on-surface mb-4">爬虫采集分布</h3>
          {spiderDistribution.length === 0 && (
            <p className="text-xs text-on-surface-variant text-center py-4">暂无爬虫采集数据</p>
          )}
          {spiderDistribution.length > 0 && (
            <div className="space-y-3">
              {spiderDistribution.map((p) => (
                <div
                  key={p.name}
                  className={`space-y-1 cursor-pointer rounded-lg px-2 py-1.5 transition-colors ${spiderFilter === p.id ? 'bg-primary/10' : 'hover:bg-surface-container-low'}`}
                  onClick={() => { setSpiderFilter(spiderFilter === p.id ? '' : p.id); setPage(1); }}
                >
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-on-surface-variant truncate max-w-[120px]">{p.name}</span>
                    <span className="text-xs font-medium text-on-surface">{p.pct}% ({formatNumber(p.count)})</span>
                  </div>
                  <div className="h-1.5 rounded-full bg-surface-container-high overflow-hidden">
                    <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${p.pct}%` }} />
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Detail Modal */}
      {selectedResult && (
        <DetailModal result={selectedResult} onClose={() => setSelectedResult(null)} avatarMap={targetAvatarMap} />
      )}
    </div>
  );
}
