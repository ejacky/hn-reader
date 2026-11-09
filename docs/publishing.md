# 转 public 之前

仓库在 2026-11-09 从 private 转成 public。这个文件记录当时怎么查的、结论是什么，以及以后新增历史时该复查什么——因为转 public 会把**全部提交历史**一起公开，包括早就删掉、只躺在历史里的文件。

## 查三样

1. **凭据**：API key、token、私钥、数据库密码。
2. **个人数据**：本地数据库、日志、截图里的真实邮箱或姓名。
3. **制品**：二进制、安装包、备份——体积之外，还要确认没有把环境变量编进去。

## 怎么查

```bash
# 1. 当前跟踪的文件里有没有 .env / 密钥
git ls-files | grep -iE '\.env|secret|credential|\.pem$|\.key$|id_rsa'

# 2. 历史上新增过的可疑路径
git log --all --diff-filter=A --name-only --pretty=format: | sort -u |
  grep -iE '\.env|secret|\.key$|\.db$|backup|\.sql'

# 3. 全历史内容扫描。提交多的时候别把 SHA 全塞进参数列表，逐条查：
for c in $(git rev-list --all); do
  git grep -I -n -E 'sk-[A-Za-z0-9]{20,}|(ghp|gho|ghu|ghs)_[A-Za-z0-9]{30,}|-----BEGIN [A-Z ]*PRIVATE KEY-----|(AKIA|AKID)[A-Za-z0-9]{12,}' "$c" -- .
done | sort -u

# 4. 历史里最大的对象（二进制、数据文件）
git rev-list --objects --all |
  git cat-file --batch-check='%(objectname) %(objectsize) %(rest)' |
  sort -k2 -n -r | head -10
```

第 3 步只认「凭据长什么样」，不认文件名——`.env` 改名了也查得出来。
已经提交过的二进制，再用字符串扫描查一遍有没有被编进去的 key（`sk-…`、`LLM_API_KEY=`、`POSTGRES_PASSWORD=`）和编译机用户名。

## 2026-11-09 的结论

- 51 个提交，全历史无凭据命中；`.env` 从未入库，三个 `.env.example` 全是占位值。
- 历史里两个 exe（36MB / 19MB）字符串扫描：无 key、无密码、无编译机用户名。
- 遗留，均不影响安全：
  - `server/hn-reader.db`（200KB）：早期本地测试的 `articles` / `comments` / `user_signals`，无账号、无邮箱；公开的只是当时的阅读信号。
  - 约 55MB 二进制（两个 exe + 三份 `extension.zip`），只是体积。
  - 清这两样要 `git filter-repo` + force push，所有 SHA 会变，issue 里引用旧 SHA 的链接（含 CI 关闭评论）会 404，暂不处理。

## 转 public 之后

- Secret scanning 与 push protection 已开启（Settings → Code security）。
- 落地页与商店详情页都链到本仓库的 `docs/PRIVACY.md` 与 issues，公开之前它们是 404。
