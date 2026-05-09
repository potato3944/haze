// 雾霾探测系统 Service Worker
// 版本号：更新时修改这个值可以强制刷新缓存
const CACHE_NAME = 'haze-app-v1';

// 需要预缓存的静态资源（离线时也能显示基本界面）
const STATIC_ASSETS = [
  '/',
  '/index.html',
  '/style.css',
  '/app.js',
  '/icon-192.png',
  '/icon-512.png',
];

// 安装：预缓存静态资源
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(STATIC_ASSETS))
  );
  self.skipWaiting();
});

// 激活：清除旧版缓存
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k)))
    )
  );
  self.clients.claim();
});

// 拦截请求：API 调用走网络，静态资源走缓存优先
self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);

  // API 请求始终走网络（需要实时数据）
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(fetch(event.request));
    return;
  }

  // 静态资源：缓存优先，缓存没有再走网络
  event.respondWith(
    caches.match(event.request).then((cached) => cached || fetch(event.request))
  );
});
