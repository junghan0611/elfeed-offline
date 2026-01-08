/**
 * Elfeed Offline - 앱 로직
 * 순수 JavaScript로 작성된 프론트엔드
 */

(function() {
  'use strict';

  // ============================================
  // 상태 관리 (State Management)
  // ============================================

  const state = {
    entries: new Map(),      // webid -> entry 데이터
    results: [],             // 현재 검색 결과 webid 목록
    selected: null,          // 현재 선택된 엔트리 webid
    searchQuery: '@30-days-ago +unread',  // 기본 검색 쿼리
    isOffline: !navigator.onLine,  // 오프라인 상태
    pendingTagUpdates: []    // 대기 중인 태그 업데이트
  };

  // ============================================
  // 유틸리티 함수
  // ============================================

  /**
   * DOM 요소 선택 헬퍼
   * @param {string} id - 요소 ID
   * @returns {HTMLElement|null}
   */
  function $(id) {
    return document.getElementById(id);
  }

  /**
   * 상태 메시지 표시
   * @param {string} msg - 표시할 메시지
   */
  function setStatus(msg) {
    const statusEl = $('status');
    if (statusEl) {
      statusEl.textContent = msg;
    }
  }

  /**
   * 네비게이션 상태 표시
   * @param {string} msg - 표시할 메시지
   */
  function setNavStatus(msg) {
    const navStatusEl = $('nav-status');
    if (navStatusEl) {
      navStatusEl.textContent = msg;
      navStatusEl.className = msg ? 'set' : 'empty';
    }
  }

  /**
   * 날짜 포맷팅 (상대 시간)
   * @param {number} timestamp - Unix 타임스탬프
   * @returns {string}
   */
  function formatDate(timestamp) {
    const date = new Date(timestamp * 1000);
    const now = new Date();
    const diff = Math.floor((now - date) / 1000);

    if (diff < 60) return '방금 전';
    if (diff < 3600) return `${Math.floor(diff / 60)}분 전`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}시간 전`;
    if (diff < 604800) return `${Math.floor(diff / 86400)}일 전`;

    // 일주일 이상이면 날짜 표시
    return date.toLocaleDateString('ko-KR', {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    });
  }

  /**
   * HTML 이스케이프
   * @param {string} str - 이스케이프할 문자열
   * @returns {string}
   */
  function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }

  // ============================================
  // API 통신
  // ============================================

  /**
   * 검색 API 호출
   * @param {string} query - 검색 쿼리
   * @returns {Promise<Array>}
   */
  async function fetchSearch(query) {
    const url = `/elfeed/search?q=${encodeURIComponent(query)}`;
    const response = await fetch(url, {
      credentials: 'same-origin'
    });

    if (!response.ok) {
      if (response.status === 401) {
        throw new Error('인증이 필요합니다. Elfeed 자격 증명으로 로그인하세요.');
      }
      if (response.status === 403 || response.status === 500) {
        throw new Error('Emacs elfeed 서버가 실행 중인지 확인하세요.');
      }
      throw new Error('검색 실패: 오프라인이거나 서버를 사용할 수 없습니다.');
    }

    return response.json();
  }

  /**
   * 콘텐츠 로드
   * @param {string} hash - 콘텐츠 해시
   * @returns {Promise<string>}
   */
  async function fetchContent(hash) {
    const url = `/elfeed/content/${hash}`;
    const response = await fetch(url, {
      credentials: 'same-origin'
    });

    if (!response.ok) {
      throw new Error('콘텐츠를 불러올 수 없습니다.');
    }

    return response.text();
  }

  /**
   * 태그 업데이트 API 호출
   * @param {string} webid - 엔트리 웹 ID
   * @param {Array<string>} add - 추가할 태그
   * @param {Array<string>} remove - 제거할 태그
   * @returns {Promise<Response>}
   */
  async function updateTags(webid, add = [], remove = []) {
    const body = {
      entries: [webid],
      add: add,
      remove: remove
    };

    const response = await fetch('/elfeed/tags', {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json'
      },
      credentials: 'same-origin',
      body: JSON.stringify(body)
    });

    return response;
  }

  // ============================================
  // UI 렌더링
  // ============================================

  /**
   * 검색 결과 렌더링
   */
  function renderResults() {
    const entriesEl = $('entries');
    const loadingEl = $('loading');

    if (!entriesEl) return;

    // 로딩 숨기기
    if (loadingEl) {
      loadingEl.classList.add('hidden');
    }

    // 결과가 없으면 빈 메시지
    if (state.results.length === 0) {
      entriesEl.innerHTML = '<div class="loading">검색 결과가 없습니다.</div>';
      return;
    }

    // 엔트리 목록 렌더링
    const html = state.results.map(webid => {
      const entry = state.entries.get(webid);
      if (!entry) return '';

      const isUnread = entry.tags && entry.tags.includes('unread');
      const isStarred = entry.tags && entry.tags.includes('starred');
      const isSelected = state.selected === webid;

      // 태그 HTML (unread, starred 제외)
      const otherTags = (entry.tags || [])
        .filter(tag => tag !== 'unread' && tag !== 'starred')
        .map(tag => `<span class="tag">${escapeHtml(tag)}</span>`)
        .join('');

      return `
        <article class="entry ${isUnread ? 'unread' : ''} ${isSelected ? 'selected' : ''}"
                 data-webid="${escapeHtml(webid)}"
                 data-hash="${escapeHtml(entry.content || '')}">
          <span class="title">${escapeHtml(entry.title)}</span>
          <div class="entry-controls">
            <button class="star ${isStarred ? 'starred' : ''}"
                    data-action="${isStarred ? 'unstar' : 'star'}"
                    title="${isStarred ? '즐겨찾기 해제' : '즐겨찾기'}">
              <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24"
                   fill="${isStarred ? 'currentColor' : 'none'}" stroke="currentColor"
                   stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M11.525 2.295a.53.53 0 0 1 .95 0l2.31 4.679a2.123 2.123 0 0 0 1.595 1.16l5.166.756a.53.53 0 0 1 .294.904l-3.736 3.638a2.123 2.123 0 0 0-.611 1.878l.882 5.14a.53.53 0 0 1-.771.56l-4.618-2.428a2.122 2.122 0 0 0-1.973 0L6.396 21.01a.53.53 0 0 1-.77-.56l.881-5.139a2.122 2.122 0 0 0-.611-1.879L2.16 9.795a.53.53 0 0 1 .294-.906l5.165-.755a2.122 2.122 0 0 0 1.597-1.16z"/>
              </svg>
            </button>
          </div>
          <span class="feed">${escapeHtml(entry.feed?.title || '')}</span>
          <span class="date">${formatDate(entry.date)}</span>
          ${otherTags ? `<div class="tags">${otherTags}</div>` : ''}
        </article>
      `;
    }).join('');

    entriesEl.innerHTML = html;

    // 이벤트 바인딩
    entriesEl.querySelectorAll('.entry').forEach(el => {
      // 엔트리 클릭
      el.addEventListener('click', (e) => {
        // 버튼 클릭은 무시
        if (e.target.closest('button')) return;
        const webid = el.dataset.webid;
        selectEntry(webid);
      });

      // 별표 버튼 클릭
      el.querySelector('.star')?.addEventListener('click', (e) => {
        e.stopPropagation();
        const webid = el.dataset.webid;
        const action = e.currentTarget.dataset.action;
        toggleStar(webid, action === 'star');
      });
    });
  }

  /**
   * 엔트리 선택 및 콘텐츠 로드
   * @param {string} webid - 엔트리 웹 ID
   */
  async function selectEntry(webid) {
    const entry = state.entries.get(webid);
    if (!entry) return;

    state.selected = webid;

    // UI 업데이트 - 선택 상태
    document.querySelectorAll('.entry').forEach(el => {
      el.classList.toggle('selected', el.dataset.webid === webid);
    });

    // 모바일에서 읽기 모드 활성화
    document.body.classList.add('reading');

    // 네비게이션 정보 업데이트
    const navTitle = $('nav-title');
    const navFeed = $('nav-feed');
    const openOriginal = $('open-original');

    if (navTitle) navTitle.textContent = entry.title || '';
    if (navFeed) navFeed.textContent = entry.feed?.title || '';
    if (openOriginal) openOriginal.href = entry.link || '#';

    // 버튼 활성화
    enableNavButtons(true);
    updateNavButtons(entry);

    // 콘텐츠 로드
    const iframe = $('content');
    if (!iframe) return;

    try {
      setNavStatus('콘텐츠 로드 중...');
      const content = await fetchContent(entry.content);

      // iframe에 콘텐츠 삽입
      const doc = iframe.contentDocument || iframe.contentWindow.document;
      doc.open();
      doc.write(`
        <!DOCTYPE html>
        <html>
        <head>
          <meta charset="utf-8">
          <meta name="viewport" content="width=device-width, initial-scale=1">
          <style>
            :root { color-scheme: light dark; }
            body {
              font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Noto Sans KR", sans-serif;
              line-height: 1.6;
              padding: 1em;
              max-width: 800px;
              margin: 0 auto;
            }
            img { max-width: 100%; height: auto; }
            pre { overflow-x: auto; padding: 1em; background: rgba(0,0,0,0.05); border-radius: 4px; }
            code { font-family: "SF Mono", Monaco, Consolas, monospace; }
            a { color: #0066cc; }
            @media (prefers-color-scheme: dark) {
              body { background: #1a1a2e; color: #e8e8e8; }
              a { color: #64b5f6; }
              pre { background: rgba(255,255,255,0.05); }
            }
          </style>
        </head>
        <body>${content}</body>
        </html>
      `);
      doc.close();

      setNavStatus('');

      // 읽지 않은 경우 자동으로 읽음 표시
      if (entry.tags?.includes('unread')) {
        markAsRead(webid);
      }
    } catch (err) {
      console.error('콘텐츠 로드 실패:', err);
      setNavStatus('콘텐츠를 불러올 수 없습니다.');
    }
  }

  /**
   * 네비게이션 버튼 활성화/비활성화
   * @param {boolean} enabled - 활성화 여부
   */
  function enableNavButtons(enabled) {
    const buttons = [
      'close-entry', 'mark-read', 'mark-unread',
      'star-entry', 'unstar-entry', 'copy-url'
    ];
    buttons.forEach(id => {
      const btn = $(id);
      if (btn) btn.disabled = !enabled;
    });
  }

  /**
   * 네비게이션 버튼 상태 업데이트
   * @param {Object} entry - 엔트리 데이터
   */
  function updateNavButtons(entry) {
    const isUnread = entry.tags?.includes('unread');
    const isStarred = entry.tags?.includes('starred');

    const markRead = $('mark-read');
    const markUnread = $('mark-unread');
    const starEntry = $('star-entry');
    const unstarEntry = $('unstar-entry');

    if (markRead) markRead.style.display = isUnread ? 'inline-flex' : 'none';
    if (markUnread) markUnread.style.display = isUnread ? 'none' : 'inline-flex';
    if (starEntry) starEntry.style.display = isStarred ? 'none' : 'inline-flex';
    if (unstarEntry) unstarEntry.style.display = isStarred ? 'inline-flex' : 'none';
  }

  /**
   * 엔트리 닫기
   */
  function closeEntry() {
    state.selected = null;
    document.body.classList.remove('reading');

    document.querySelectorAll('.entry.selected').forEach(el => {
      el.classList.remove('selected');
    });

    enableNavButtons(false);

    const navTitle = $('nav-title');
    const navFeed = $('nav-feed');
    if (navTitle) navTitle.textContent = '';
    if (navFeed) navFeed.textContent = '';

    // iframe 초기화
    const iframe = $('content');
    if (iframe) {
      const doc = iframe.contentDocument || iframe.contentWindow.document;
      doc.open();
      doc.write('');
      doc.close();
    }
  }

  // ============================================
  // 태그 조작
  // ============================================

  /**
   * 읽음으로 표시
   * @param {string} webid - 엔트리 웹 ID
   */
  async function markAsRead(webid) {
    const entry = state.entries.get(webid);
    if (!entry) return;

    // 로컬 상태 업데이트
    entry.tags = (entry.tags || []).filter(t => t !== 'unread');
    state.entries.set(webid, entry);

    // UI 업데이트
    const entryEl = document.querySelector(`.entry[data-webid="${webid}"]`);
    if (entryEl) entryEl.classList.remove('unread');
    if (state.selected === webid) updateNavButtons(entry);

    // 서버에 전송
    try {
      await updateTags(webid, [], ['unread']);
    } catch (err) {
      console.error('태그 업데이트 실패:', err);
      // 오프라인 대기열에 추가
      state.pendingTagUpdates.push({ webid, add: [], remove: ['unread'] });
    }
  }

  /**
   * 읽지 않음으로 표시
   * @param {string} webid - 엔트리 웹 ID
   */
  async function markAsUnread(webid) {
    const entry = state.entries.get(webid);
    if (!entry) return;

    // 로컬 상태 업데이트
    if (!entry.tags) entry.tags = [];
    if (!entry.tags.includes('unread')) entry.tags.push('unread');
    state.entries.set(webid, entry);

    // UI 업데이트
    const entryEl = document.querySelector(`.entry[data-webid="${webid}"]`);
    if (entryEl) entryEl.classList.add('unread');
    if (state.selected === webid) updateNavButtons(entry);

    // 서버에 전송
    try {
      await updateTags(webid, ['unread'], []);
    } catch (err) {
      console.error('태그 업데이트 실패:', err);
      state.pendingTagUpdates.push({ webid, add: ['unread'], remove: [] });
    }
  }

  /**
   * 즐겨찾기 토글
   * @param {string} webid - 엔트리 웹 ID
   * @param {boolean} star - true면 추가, false면 제거
   */
  async function toggleStar(webid, star) {
    const entry = state.entries.get(webid);
    if (!entry) return;

    // 로컬 상태 업데이트
    if (!entry.tags) entry.tags = [];
    if (star) {
      if (!entry.tags.includes('starred')) entry.tags.push('starred');
    } else {
      entry.tags = entry.tags.filter(t => t !== 'starred');
    }
    state.entries.set(webid, entry);

    // UI 업데이트
    renderResults();
    if (state.selected === webid) updateNavButtons(entry);

    // 서버에 전송
    try {
      if (star) {
        await updateTags(webid, ['starred'], []);
      } else {
        await updateTags(webid, [], ['starred']);
      }
    } catch (err) {
      console.error('태그 업데이트 실패:', err);
      if (star) {
        state.pendingTagUpdates.push({ webid, add: ['starred'], remove: [] });
      } else {
        state.pendingTagUpdates.push({ webid, add: [], remove: ['starred'] });
      }
    }
  }

  /**
   * 모든 결과 읽음으로 표시
   */
  async function markAllAsRead() {
    const count = state.results.length;
    if (count === 0) {
      setStatus('읽음 표시할 항목이 없습니다.');
      return;
    }

    // 확인 대화상자
    let confirmed = false;
    if (count < 10) {
      confirmed = confirm(`${count}개 항목을 모두 읽음으로 표시하시겠습니까?`);
    } else {
      const input = prompt(
        `${count}개 항목을 모두 읽음으로 표시합니다. 확인하려면 "READ-${count}"를 입력하세요.`
      );
      confirmed = input === `READ-${count}`;
    }

    if (!confirmed) {
      setStatus('취소되었습니다.');
      return;
    }

    setStatus('모두 읽음으로 표시 중...');

    // 모든 항목 처리
    for (const webid of state.results) {
      const entry = state.entries.get(webid);
      if (entry && entry.tags?.includes('unread')) {
        entry.tags = entry.tags.filter(t => t !== 'unread');
        state.entries.set(webid, entry);
      }
    }

    // UI 업데이트
    renderResults();
    setStatus(`${count}개 항목을 읽음으로 표시했습니다.`);

    // 서버에 일괄 전송 (개별 요청)
    for (const webid of state.results) {
      try {
        await updateTags(webid, [], ['unread']);
      } catch (err) {
        console.error('태그 업데이트 실패:', webid, err);
      }
    }
  }

  // ============================================
  // 검색
  // ============================================

  /**
   * 검색 실행
   */
  async function doSearch() {
    const qInput = $('q');
    if (!qInput) return;

    const query = qInput.value.trim();
    if (!query) {
      setStatus('검색어를 입력하세요.');
      return;
    }

    state.searchQuery = query;
    setStatus('검색 중...');

    const loadingEl = $('loading');
    const entriesEl = $('entries');
    if (loadingEl) loadingEl.classList.remove('hidden');
    if (entriesEl) entriesEl.innerHTML = '';

    try {
      const data = await fetchSearch(query);

      // 상태 업데이트
      state.entries.clear();
      state.results = [];

      data.forEach(entry => {
        if (entry.webid && entry.content) {
          state.entries.set(entry.webid, entry);
          state.results.push(entry.webid);
        }
      });

      // 현재 선택된 항목이 결과에 없으면 닫기
      if (state.selected && !state.results.includes(state.selected)) {
        closeEntry();
      }

      renderResults();
      setStatus(`${state.results.length}개 항목을 찾았습니다.`);

    } catch (err) {
      console.error('검색 실패:', err);
      setStatus(err.message || '검색에 실패했습니다.');
      if (loadingEl) loadingEl.classList.add('hidden');
    }
  }

  /**
   * 검색어에 토큰 추가/제거
   * @param {string} token - 추가/제거할 토큰 (예: "+unread")
   */
  function toggleSearchToken(token) {
    const qInput = $('q');
    if (!qInput) return;

    let query = qInput.value;
    if (query.includes(token)) {
      query = query.replace(token, '').replace(/\s+/g, ' ').trim();
    } else {
      query = query + ' ' + token;
    }
    qInput.value = query;
    doSearch();
  }

  // ============================================
  // 오프라인 기능
  // ============================================

  /**
   * 현재 결과 오프라인 저장
   */
  function saveForOffline() {
    if (!('serviceWorker' in navigator)) {
      setStatus('Service Worker를 사용할 수 없습니다.');
      return;
    }

    const hashes = state.results
      .slice(0, 100)  // 최대 100개
      .map(webid => state.entries.get(webid)?.content)
      .filter(Boolean);

    if (hashes.length === 0) {
      setStatus('저장할 항목이 없습니다.');
      return;
    }

    setStatus(`${hashes.length}개 항목 저장 요청 중...`);

    // Service Worker에 메시지 전송
    navigator.serviceWorker.controller?.postMessage({
      type: 'PREFETCH_REQUEST',
      hashes: hashes
    });
  }

  /**
   * 캐시 삭제
   */
  async function clearCache() {
    if (!confirm('오프라인 캐시를 삭제하시겠습니까?')) {
      return;
    }

    if ('caches' in window) {
      try {
        const keys = await caches.keys();
        for (const key of keys) {
          await caches.delete(key);
        }
        setStatus('캐시가 삭제되었습니다.');
      } catch (err) {
        console.error('캐시 삭제 실패:', err);
        setStatus('캐시 삭제에 실패했습니다.');
      }
    }
  }

  /**
   * URL 클립보드에 복사
   */
  async function copyUrl() {
    if (!state.selected) return;

    const entry = state.entries.get(state.selected);
    if (!entry?.link) return;

    try {
      await navigator.clipboard.writeText(entry.link);
      setNavStatus('링크가 복사되었습니다.');
      setTimeout(() => setNavStatus(''), 2000);
    } catch (err) {
      console.error('복사 실패:', err);
      setNavStatus('복사에 실패했습니다.');
    }
  }

  // ============================================
  // 이벤트 핸들러 설정
  // ============================================

  function setupEventHandlers() {
    // 검색 폼
    $('search-form')?.addEventListener('submit', (e) => {
      e.preventDefault();
      doSearch();
    });

    // 검색 입력 변경
    $('q')?.addEventListener('change', () => {
      state.searchQuery = $('q').value;
    });

    // 토글 버튼들
    $('toggle-unread')?.addEventListener('click', () => toggleSearchToken('+unread'));
    $('toggle-starred')?.addEventListener('click', () => toggleSearchToken('+starred'));

    // 컨트롤 버튼들
    $('mark-all-read')?.addEventListener('click', markAllAsRead);
    $('save-offline')?.addEventListener('click', saveForOffline);
    $('clear-cache')?.addEventListener('click', clearCache);

    // 네비게이션 버튼들
    $('close-entry')?.addEventListener('click', closeEntry);
    $('mark-read')?.addEventListener('click', () => {
      if (state.selected) markAsRead(state.selected);
    });
    $('mark-unread')?.addEventListener('click', () => {
      if (state.selected) markAsUnread(state.selected);
    });
    $('star-entry')?.addEventListener('click', () => {
      if (state.selected) toggleStar(state.selected, true);
    });
    $('unstar-entry')?.addEventListener('click', () => {
      if (state.selected) toggleStar(state.selected, false);
    });
    $('copy-url')?.addEventListener('click', copyUrl);

    // 키보드 단축키
    document.addEventListener('keydown', (e) => {
      // ESC: 엔트리 닫기
      if (e.key === 'Escape' && state.selected) {
        closeEntry();
      }
      // j/k: 다음/이전 엔트리
      if (e.key === 'j' || e.key === 'k') {
        const currentIndex = state.results.indexOf(state.selected);
        let newIndex;
        if (e.key === 'j') {
          newIndex = currentIndex < state.results.length - 1 ? currentIndex + 1 : 0;
        } else {
          newIndex = currentIndex > 0 ? currentIndex - 1 : state.results.length - 1;
        }
        if (state.results[newIndex]) {
          selectEntry(state.results[newIndex]);
        }
      }
    });

    // 온라인/오프라인 상태 변경
    window.addEventListener('online', updateOnlineStatus);
    window.addEventListener('offline', updateOnlineStatus);

    // 해시 변경 (브라우저 히스토리)
    window.addEventListener('hashchange', () => {
      const hash = window.location.hash.slice(1);
      if (hash) {
        const params = new URLSearchParams(hash);
        const q = params.get('q');
        if (q) {
          $('q').value = q;
          doSearch();
        }
      }
    });
  }

  /**
   * 온라인/오프라인 상태 업데이트
   */
  function updateOnlineStatus() {
    state.isOffline = !navigator.onLine;
    const indicator = $('offline-indicator');
    if (indicator) {
      indicator.classList.toggle('offline', state.isOffline);
    }
  }

  // ============================================
  // Service Worker 메시지 핸들러
  // ============================================

  function setupServiceWorkerMessages() {
    if (!('serviceWorker' in navigator)) return;

    navigator.serviceWorker.addEventListener('message', (event) => {
      const data = event.data;
      if (!data || !data.type) return;

      switch (data.type) {
        case 'PREFETCH_STARTED':
          setStatus(`저장 시작: 0/${data.total}`);
          break;
        case 'PREFETCH_PROGRESS':
          setStatus(`저장 중: ${data.done}/${data.total}`);
          break;
        case 'PREFETCH_DONE':
          setStatus(`${data.total}개 항목 저장 완료`);
          break;
        case 'PREFETCH_ERROR':
          setStatus(`저장 오류: ${data.msg}`);
          break;
        case 'CACHE_CLEARED':
          setStatus('캐시가 삭제되었습니다.');
          break;
      }
    });
  }

  // ============================================
  // 초기화
  // ============================================

  function init() {
    console.log('Elfeed Offline 앱 초기화');

    // 이벤트 핸들러 설정
    setupEventHandlers();
    setupServiceWorkerMessages();

    // 온라인 상태 확인
    updateOnlineStatus();

    // URL 해시에서 쿼리 복원
    const hash = window.location.hash.slice(1);
    if (hash) {
      const params = new URLSearchParams(hash);
      const q = params.get('q');
      if (q) {
        $('q').value = q;
        state.searchQuery = q;
      }
    }

    // 초기 검색 실행
    doSearch();

    // 버튼 초기 비활성화
    enableNavButtons(false);
  }

  // DOM 로드 완료 시 초기화
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

})();
