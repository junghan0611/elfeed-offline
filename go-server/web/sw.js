/**
 * Elfeed Offline - Service Worker
 * 오프라인 캐싱 및 백그라운드 동기화
 */

// ============================================
// 캐시 설정
// ============================================

const CACHE_VERSION = 'v1';
const CACHE_SHELL = `shell-${CACHE_VERSION}`;    // 정적 파일 캐시
const CACHE_CONTENT = `content-${CACHE_VERSION}`; // elfeed 콘텐츠 캐시
const CACHE_SEARCH = `search-${CACHE_VERSION}`;   // 검색 결과 캐시

// 캐시할 정적 파일 목록
const SHELL_FILES = [
  '/',
  '/index.html',
  '/css/app.css',
  '/js/app.js',
  '/manifest.webmanifest'
];

// ============================================
// 설치 이벤트
// ============================================

self.addEventListener('install', (event) => {
  console.log('[SW] 설치 중...');

  event.waitUntil(
    caches.open(CACHE_SHELL)
      .then(cache => {
        console.log('[SW] 정적 파일 캐싱');
        return cache.addAll(SHELL_FILES);
      })
      .then(() => {
        console.log('[SW] 설치 완료');
        // 대기 중인 SW 즉시 활성화
        return self.skipWaiting();
      })
      .catch(err => {
        console.error('[SW] 설치 실패:', err);
      })
  );
});

// ============================================
// 활성화 이벤트
// ============================================

self.addEventListener('activate', (event) => {
  console.log('[SW] 활성화 중...');

  event.waitUntil(
    Promise.all([
      // 이전 버전 캐시 삭제
      caches.keys().then(keys => {
        return Promise.all(
          keys
            .filter(key => {
              return key !== CACHE_SHELL &&
                     key !== CACHE_CONTENT &&
                     key !== CACHE_SEARCH;
            })
            .map(key => {
              console.log('[SW] 이전 캐시 삭제:', key);
              return caches.delete(key);
            })
        );
      }),
      // 모든 클라이언트 제어
      self.clients.claim()
    ]).then(() => {
      console.log('[SW] 활성화 완료');
    })
  );
});

// ============================================
// Fetch 이벤트 - 캐시 전략
// ============================================

self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);

  // 정적 파일 - Cache First (with refresh)
  if (SHELL_FILES.includes(url.pathname) || url.pathname === '/') {
    event.respondWith(cacheFirstWithRefresh(event.request, CACHE_SHELL));
    return;
  }

  // elfeed 콘텐츠 - Cache First (콘텐츠 해시로 식별되므로 변하지 않음)
  if (url.pathname.startsWith('/elfeed/content/')) {
    event.respondWith(cacheFirst(event.request, CACHE_CONTENT));
    return;
  }

  // elfeed 검색 - Cache First with Refresh (백그라운드 업데이트)
  if (url.pathname.startsWith('/elfeed/search')) {
    event.respondWith(cacheFirstWithRefresh(event.request, CACHE_SEARCH));
    return;
  }

  // 기타 요청 - Network First
  event.respondWith(networkFirst(event.request));
});

// ============================================
// 캐시 전략 함수들
// ============================================

/**
 * Cache First 전략
 * 캐시에 있으면 캐시 반환, 없으면 네트워크 요청 후 캐시
 */
async function cacheFirst(request, cacheName) {
  const cache = await caches.open(cacheName);
  const cached = await cache.match(request);

  if (cached) {
    return cached;
  }

  try {
    const response = await fetch(request);
    if (response.ok) {
      cache.put(request, response.clone());
    }
    return response;
  } catch (err) {
    console.error('[SW] 네트워크 요청 실패:', err);
    // 오프라인 폴백
    return new Response('Offline', { status: 503 });
  }
}

/**
 * Cache First with Refresh 전략
 * 캐시 반환하면서 백그라운드로 업데이트
 */
async function cacheFirstWithRefresh(request, cacheName) {
  const cache = await caches.open(cacheName);
  const cached = await cache.match(request);

  // 백그라운드 업데이트 (결과 기다리지 않음)
  const fetchPromise = fetch(request)
    .then(response => {
      if (response.ok) {
        cache.put(request, response.clone());
      }
      return response;
    })
    .catch(err => {
      console.log('[SW] 백그라운드 업데이트 실패:', err);
    });

  if (cached) {
    return cached;
  }

  // 캐시 없으면 네트워크 응답 대기
  try {
    return await fetchPromise;
  } catch (err) {
    return new Response('Offline', { status: 503 });
  }
}

/**
 * Network First 전략
 * 네트워크 먼저 시도, 실패하면 캐시 반환
 */
async function networkFirst(request) {
  try {
    const response = await fetch(request);
    return response;
  } catch (err) {
    console.log('[SW] 네트워크 실패, 캐시 확인:', request.url);
    const cache = await caches.open(CACHE_CONTENT);
    const cached = await cache.match(request);
    if (cached) {
      return cached;
    }
    return new Response('Offline', { status: 503 });
  }
}

// ============================================
// 메시지 핸들러 (클라이언트 통신)
// ============================================

self.addEventListener('message', async (event) => {
  const data = event.data;
  if (!data || !data.type) return;

  switch (data.type) {
    case 'PREFETCH_REQUEST':
      await prefetchContent(data.hashes);
      break;

    case 'CLEAR_CACHE':
      await clearContentCache();
      break;
  }
});

/**
 * 콘텐츠 프리페치
 * @param {string[]} hashes - 콘텐츠 해시 목록
 */
async function prefetchContent(hashes) {
  if (!hashes || hashes.length === 0) return;

  const total = hashes.length;
  let done = 0;

  // 모든 클라이언트에 시작 알림
  notifyClients({ type: 'PREFETCH_STARTED', total });

  const cache = await caches.open(CACHE_CONTENT);

  // 동시에 4개씩 처리
  const concurrency = 4;
  const chunks = [];
  for (let i = 0; i < hashes.length; i += concurrency) {
    chunks.push(hashes.slice(i, i + concurrency));
  }

  for (const chunk of chunks) {
    await Promise.all(
      chunk.map(async (hash) => {
        try {
          const url = `/elfeed/content/${hash}`;
          const request = new Request(url);

          // 캐시에 없으면 가져오기
          const cached = await cache.match(request);
          if (!cached) {
            const response = await fetch(request);
            if (response.ok) {
              await cache.put(request, response);
            }
          }

          done++;
          notifyClients({ type: 'PREFETCH_PROGRESS', done, total });
        } catch (err) {
          console.error('[SW] 프리페치 실패:', hash, err);
          notifyClients({ type: 'PREFETCH_ERROR', msg: hash });
        }
      })
    );
  }

  notifyClients({ type: 'PREFETCH_DONE', total: done });
}

/**
 * 콘텐츠 캐시 삭제
 */
async function clearContentCache() {
  try {
    await caches.delete(CACHE_CONTENT);
    await caches.delete(CACHE_SEARCH);
    notifyClients({ type: 'CACHE_CLEARED', success: true });
  } catch (err) {
    console.error('[SW] 캐시 삭제 실패:', err);
    notifyClients({ type: 'CACHE_CLEARED', success: false });
  }
}

/**
 * 모든 클라이언트에 메시지 전송
 * @param {Object} message - 전송할 메시지
 */
async function notifyClients(message) {
  const clients = await self.clients.matchAll();
  clients.forEach(client => {
    client.postMessage(message);
  });
}
