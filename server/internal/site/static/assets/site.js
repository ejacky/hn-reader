/* HN Reader landing page: CTA wiring + EN/ZH copy swap.
   English lives in the HTML so the page still reads correctly with JS disabled
   and for crawlers; this file only overrides the text when the visitor is on
   Chinese. No network calls, no third-party scripts — the CSP forbids both. */

(function () {
  'use strict';

  var CONFIG = {
    // The live Chrome Web Store listing. Empty would mean "not published yet"
    // and every install button would fall back to GitHub Releases, with a note
    // explaining why.
    storeUrl: 'https://chromewebstore.google.com/detail/hn-reader-ai-reading-assi/bjjgpeekaedbhcpglinhfcdkglmfmbkm',
    releasesUrl: 'https://github.com/ejacky/hn-reader/releases/latest'
  };

  var ZH = {
    'nav.features': '功能',
    'nav.how': '怎么用',
    'nav.privacy': '隐私',
    'nav.faq': '常见问题',

    'cta.install': '添加到 Chrome',
    'cta.install.alt': '从 GitHub 下载',
    'cta.source': '在 GitHub 上看源码',

    'hero.eyebrow': '写给每天读 Hacker News 的人',
    'hero.title': '用<span class="hl">一半的时间</span>读完 Hacker News。',
    'hero.lede': '每篇文章、每条评论旁都有一键 AI 摘要；扩展还会学习你真正读了什么，把你可能关心的帖子标出来。不用注册、不用 API Key，装完打开 Hacker News 就能用。',
    'hero.meta': '支持 Chrome 与 Edge · 每月 50 条免费摘要 · 数据随时可导出、可删除',
    'hero.note': 'Chrome 应用商店版本正在审核中，暂可从 GitHub Releases 下载安装包。',

    'shot.caption': '每篇文章都有 <code>[summary]</code> 链接，点一下就看到重点，不用先啃开头。',

    'features.title': '它做什么',
    'features.lede': '就四件事，每件都做到底。没有多余的按钮。',

    'f1.title': '在你已经在读的地方给摘要',
    'f1.body': '每个标题旁一个 <code>[summary]</code>，每条评论旁一个 <code>[summarize]</code>。点一下，三行看完，一秒决定要不要点进去。',
    'f2.title': '会学习的高亮',
    'f2.body': '扩展会记录你点开了什么、收藏了什么、在哪类文章里停留更久，然后把可能对你有用的帖子用橙色边框标出来。',
    'f3.title': '不用任何配置',
    'f3.body': '云模式自动创建匿名账户：不用 API Key、不用装服务端、不用填表。每月 50 条摘要，免费。',
    'f4.title': '数据由你说删就删',
    'f4.body': '不收集姓名和邮箱。弹窗里可以导出数据，也可以直接注销账户。想完全本地跑？服务端是开源的，自己起一个就行。',

    'how.title': '怎么用',
    'how.lede': '三步，第三步就是继续读。',

    's1.title': '装扩展',
    's1.body': '从 Chrome 应用商店安装，Chrome 和 Edge 都能用，只在 news.ycombinator.com 上运行。',
    's2.title': '打开 Hacker News',
    's2.body': '列表页和评论页里会直接出现摘要入口，站点本身其他部分没有任何改动。',
    's3.title': '点开，读',
    's3.body': '你的每次点击都在训练兴趣模型，用得越久，高亮越准。',

    'privacy.title': '隐私，一屏说完',
    'privacy.lede': '一个阅读工具不需要知道你是谁，这个也不问。',

    'p1': '你的账户就是一串匿名随机 token，没有姓名、邮箱、密码。',
    'p2': '文章和评论文本只在你请求摘要时发送给模型提供商，用于生成这一条摘要。',
    'p3': '在扩展弹窗里可以导出全部数据，也可以删除账户及其数据。',
    'p4': '想完全本地化？服务端开源，配自己的 Key 跑在自己机器上。',
    'p5': '没有广告，不出售数据，不用你的阅读记录训练模型。',
    'privacy.link': '完整隐私政策 →',

    'price.title': '免费，也不藏着限制',
    'price.lede': '目前只有一档。额度更大的付费版在路上，现在能用的东西不会被收回。',
    'price.period': '每月，长期免费',
    'price.l1': '每月 50 条 AI 摘要',
    'price.l2': '额度用完后，已生成的摘要仍然可以打开',
    'price.l3': '个性化高亮与推荐',
    'price.l4': '本地模式：不限条数，只付自己的模型费用',
    'price.l5': '数据导出与账户删除',

    'faq.title': '常见问题',

    'q1': '它和 Hacker News、Y Combinator 有关系吗？',
    'a1': '没有。HN Reader 是跑在 news.ycombinator.com 之上的独立扩展，未获得 Y Combinator 的任何背书。',
    'q2': '需要自己有 OpenAI 的 Key 吗？',
    'a2': '不需要。云模式默认可用：扩展会拿到一个匿名 token 来调用我们的服务。如果你想用自己的模型，切换到本地模式，把扩展指向你自己起的服务即可。',
    'q3': '支持哪些浏览器？',
    'a3': 'Chrome 和 Edge，都是 Manifest V3。暂不支持 Firefox。',
    'q4': '50 条用完了会怎样？',
    'a4': '已经生成过的摘要照常从缓存打开。额度每月重置；不想等的话，切到本地模式用自己的 Key 可以立刻继续。',
    'q5': '我的阅读数据存在哪？',
    'a5': '阅读信号和生成的摘要存在位于上海的服务端 PostgreSQL 里，直到你删除为止；文章正文不做长期存储。细节见隐私政策。',
    'q6': '可以自己部署吗？',
    'a6': '可以。本地模式后端是 Go + SQLite，从仓库里编译出来，扩展连 http://localhost:8080 即可。',

    'foot.privacy': '隐私',
    'foot.support': '反馈',
    'foot.note': '独立项目，与 Y Combinator、Hacker News 无关。'
  };

  var TITLES = {
    en: 'HN Reader — AI summaries and personalized highlights for Hacker News',
    zh: 'HN Reader — 用一半时间读完 Hacker News'
  };
  var DESCRIPTIONS = {
    en: 'One-click AI summaries on every Hacker News post and comment, plus highlights that learn what you actually read. Free, anonymous, no API key.',
    zh: '在 Hacker News 每篇文章和评论旁一键生成 AI 摘要，并按你真实的阅读习惯高亮可能关心的帖子。免费、匿名、无需 API Key。'
  };

  var nodes = document.querySelectorAll('[data-i18n]');
  var originals = [];
  for (var i = 0; i < nodes.length; i++) {
    originals.push(nodes[i].innerHTML);
  }

  var toggle = document.getElementById('lang-toggle');
  var storeNote = document.getElementById('store-note');
  var lang = 'en';

  function ctaHref() {
    return CONFIG.storeUrl || CONFIG.releasesUrl;
  }

  function ctaText(l) {
    if (CONFIG.storeUrl) {
      return l === 'zh' ? ZH['cta.install'] : 'Add to Chrome';
    }
    return l === 'zh' ? ZH['cta.install.alt'] : 'Download from GitHub';
  }

  function apply(l) {
    lang = l === 'zh' ? 'zh' : 'en';
    var root = document.documentElement;
    root.setAttribute('data-lang', lang);
    root.setAttribute('lang', lang === 'zh' ? 'zh-CN' : 'en');

    for (var j = 0; j < nodes.length; j++) {
      var key = nodes[j].getAttribute('data-i18n');
      nodes[j].innerHTML = lang === 'zh' && ZH[key] ? ZH[key] : originals[j];
    }

    var labels = document.querySelectorAll('[data-cta-label]');
    for (var k = 0; k < labels.length; k++) {
      labels[k].textContent = ctaText(lang);
    }

    var ctas = document.querySelectorAll('[data-cta="primary"]');
    for (var m = 0; m < ctas.length; m++) {
      ctas[m].setAttribute('href', ctaHref());
    }

    if (storeNote) {
      storeNote.hidden = !!CONFIG.storeUrl;
    }

    document.title = TITLES[lang];
    var meta = document.querySelector('meta[name="description"]');
    if (meta) {
      meta.setAttribute('content', DESCRIPTIONS[lang]);
    }

    if (toggle) {
      toggle.textContent = lang === 'zh' ? 'EN' : '中文';
      toggle.setAttribute('aria-label', lang === 'zh' ? 'Switch to English' : '切换到中文');
    }

    try {
      localStorage.setItem('hnr-lang', lang);
    } catch (e) {
      /* private mode — the choice just will not persist */
    }
  }

  var saved = null;
  try {
    saved = localStorage.getItem('hnr-lang');
  } catch (e) {
    saved = null;
  }
  var initial = saved || ((navigator.language || 'en').toLowerCase().indexOf('zh') === 0 ? 'zh' : 'en');

  if (toggle) {
    toggle.addEventListener('click', function () {
      apply(lang === 'zh' ? 'en' : 'zh');
    });
  }

  apply(initial);
})();
