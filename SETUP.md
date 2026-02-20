# WUT Release Setup Guide

คู่มือการตั้งค่า GitHub และแพลตฟอร์มอื่นๆ เพื่อให้ระบบ Release ทำงานได้สมบูรณ์

---

## 📋 สรุปสิ่งที่ต้องสร้าง/ตั้งค่าทั้งหมด

### 1️⃣ GitHub Repositories (ต้องสร้างเพิ่ม)

ต้องสร้าง repositories แยกอีก **3 อัน** นอกเหนือจาก main repo (`wut`):

| Repository | จุดประสงค์ | สิทธิ์ที่ต้องการ |
|------------|-----------|-----------------|
| `thirawat27/homebrew-wut` | Homebrew Tap สำหรับ macOS/Linux | ให้ GITHUB_TOKEN เขียนได้ |
| `thirawat27/scoop-wut` | Scoop Bucket สำหรับ Windows | ให้ GITHUB_TOKEN เขียนได้ |
| `thirawat27/winget-pkgs` (fork) | WinGet Package (auto fork) | ส่ง PR ไป microsoft/winget-pkgs |

#### วิธีสร้าง:
1. ไปที่ GitHub → New Repository
2. ตั้งชื่อ: `homebrew-wut` และ `scoop-wut`
3. เลือก **Public**
4. ✅ Initialize with README (optional)

---

### 2️⃣ GitHub Secrets (ตั้งค่าใน Repository Settings)

ไปที่ **Settings → Secrets and variables → Actions** แล้วเพิ่ม:

| Secret | คำอธิบาย | ได้มาจากไหน |
|--------|---------|-------------|
| `GITHUB_TOKEN` | สร้างอัตโนมัติโดย GitHub | ✅ มีอยู่แล้ว ไม่ต้องสร้าง |
| `HOMEBREW_TAP_GITHUB_TOKEN` | Push ไป homebrew-wut repo | สร้าง Personal Access Token (PAT) |
| `SCOOP_BUCKET_GITHUB_TOKEN` | Push ไป scoop-wut repo | สร้าง PAT |
| `CHOCOLATEY_API_KEY` | Publish ไป Chocolatey | สมัครที่ chocolatey.org |
| `WINGET_TOKEN` | ส่ง PR ไป winget-pkgs | สร้าง PAT |
| `GPG_FINGERPRINT` | Sign release (optional) | สร้าง GPG key |

#### วิธีสร้าง Personal Access Token (PAT):

1. GitHub → Settings → Developer settings → Personal access tokens → **Tokens (classic)**
2. Click **Generate new token (classic)**
3. ตั้งชื่อ: `WUT Release Token`
4. เลือก Expiration: **No expiration** (หรือตามต้องการ)
5. เลือก Scopes:
   - ✅ `repo` (full control of private repositories)
   - ✅ `write:packages` (upload packages)
   - ✅ `read:packages` (download packages)
6. Click **Generate token**
7. **⚠️ คัดลอก token ทันที** (แสดงครั้งเดียว)
8. นำไปใส่ใน GitHub Secrets

---

### 3️⃣ GitHub Container Registry (GHCR)

ต้องเปิดใช้งานและตั้งค่า permissions:

1. ไปที่ **Settings → Packages and features → Package settings**
2. เปิด **"Inherit access from source repository"** หรือ
3. ตั้งค่าให้ repository มีสิทธิ์ `packages: write`

#### ทดสอบ Docker Login:
```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u thirawat27 --password-stdin
```

---

### 4️⃣ Chocolatey (chocolatey.org)

1. สมัครบัญชีที่ [community.chocolatey.org](https://community.chocolatey.org/)
2. ยืนยันอีเมล
3. ไปที่ **Account → API Keys**
4. Click **+ Create API Key**
5. ตั้งชื่อ: `WUT Package`
6. คัดลอก API Key
7. นำไปใส่ใน GitHub Secrets ชื่อ `CHOCOLATEY_API_KEY`

---

### 5️⃣ WinGet (Windows Package Manager)

ใช้ GitHub Actions อัตโนมัติ (อยู่ใน `.github/workflows/release.yml` แล้ว)

- ต้องมี `WINGET_TOKEN` ที่มีสิทธิ์ fork และส่ง PR ไป `microsoft/winget-pkgs`
- PAT ต้องมี scope: `public_repo`

---

### 6️⃣ GPG Signing (Optional)

หากต้องการ sign releases:

```bash
# สร้าง GPG key
gpg --full-generate-key

# เลือก: RSA and RSA, 4096 bits, ไม่มีวันหมดอายุ
# ใส่ชื่อและอีเมล

# ดู fingerprint
gpg --list-secret-keys --keyid-format LONG

# ตัวอย่าง output:
# sec   rsa4096/ABCD1234EFGH5678 2024-01-01 [SC]
#       A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6Q7R8S9T0
# uid                 [ultimate] Your Name <email@example.com>

# ค่า fingerprint คือ: A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6Q7R8S9T0
# นำไปใส่ใน GitHub Secrets ชื่อ GPG_FINGERPRINT

# Export private key (สำหรับ GitHub Actions - optional)
gpg --export-secret-keys --armor YOUR_KEY_ID > private.key
```

---

## 🚀 ขั้นตอนการสร้าง Release

เมื่อตั้งค่าทั้งหมดเสร็จแล้ว การสร้าง Release ทำได้ 2 วิธี:

### วิธีที่ 1: Push Tag (แนะนำ)

```bash
# Commit การเปลี่ยนแปลง
git add .
git commit -m "feat: prepare for v1.0.0 release"

# สร้าง tag
git tag -a v1.0.0 -m "Release v1.0.0 - Initial stable release"

# Push tag
git push origin v1.0.0
```

GitHub Actions จะทำงานอัตโนมัติทันที

### วิธีที่ 2: Manual Trigger

1. ไปที่ **Actions → Release → Run workflow**
2. Click **Run workflow**
3. ใส่ version เช่น `v1.0.0`
4. เลือกเป็น prerelease หรือไม่
5. Click **Run workflow**

---

## ✅ GitHub Actions ทำอะไรบ้าง

เมื่อสร้าง Release ระบบจะทำงานอัตโนมัติ:

1. ✅ **Run Tests** - รัน tests ทั้งหมด
2. ✅ **Build Binaries** - Build สำหรับทุก platform (Windows, macOS, Linux, BSD)
3. ✅ **Generate Completions** - สร้าง shell completions
4. ✅ **Create GitHub Release** - พร้อม changelog
5. ✅ **Publish to Homebrew** - Push formula ไป homebrew-wut
6. ✅ **Publish to Scoop** - Push manifest ไป scoop-wut
7. ✅ **Publish to Chocolatey** - Push package ไป chocolatey.org
8. ✅ **Build Docker Images** - Multi-arch (amd64, arm64) แล้ว push ไป GHCR
9. ✅ **Generate SBOM** - Software Bill of Materials
10. ✅ **Publish to WinGet** - ส่ง PR ไป microsoft/winget-pkgs (ถ้ามี token)

---

## 📁 Assets (ควรมี)

ไฟล์ที่ควรมีใน repository:

```
wut/
├── assets/
│   └── icon.png          # Icon สำหรับ Chocolatey (ขนาดแนะนำ: 128x128 หรือ 256x256)
├── completions/          # Auto-generated โดย GitHub Actions
│   ├── wut.bash
│   ├── _wut
│   └── wut.fish
└── ...
```

---

## 🔧 แก้ไขปัญหาเบื้องต้น

### Homebrew/Scoop push ไม่ได้

```bash
# ตรวจสอบว่า token มีสิทธิ์หรือไม่
# ไปที่ repo → Settings → Manage access → Actions secrets

# ลองสร้าง repo ใหม่แล้วเพิ่ม collaborator
```

### Docker login ไม่ได้

```bash
# ตรวจสอบว่า GITHUB_TOKEN มีสิทธิ์ packages:write
# ไปที่ Settings → Actions → General → Workflow permissions
# เลือก "Read and write permissions"
```

### Chocolatey push ไม่ได้

```bash
# ตรวจสอบว่า package ซ้ำหรือไม่ (version ต้องไม่ซ้ำ)
# ตรวจสอบว่า API key ถูกต้อง
# ตรวจสอบว่า nupkg file ถูกสร้างถูกต้อง
```

---

## 📝 Checklist ก่อน Release ครั้งแรก

- [ ] สร้าง repo `thirawat27/homebrew-wut` (public)
- [ ] สร้าง repo `thirawat27/scoop-wut` (public)
- [ ] สร้าง Personal Access Token (PAT) ด้วย scope `repo`, `write:packages`
- [ ] เพิ่ม `HOMEBREW_TAP_GITHUB_TOKEN` ใน GitHub Secrets
- [ ] เพิ่ม `SCOOP_BUCKET_GITHUB_TOKEN` ใน GitHub Secrets
- [ ] สมัคร Chocolatey → เอา API Key → เพิ่ม `CHOCOLATEY_API_KEY`
- [ ] เพิ่ม `WINGET_TOKEN` ใน GitHub Secrets (สำหรับ WinGet)
- [ ] ตรวจสอบ GitHub Container Registry permissions
- [ ] (Optional) ใส่ icon ที่ `assets/icon.png`
- [ ] (Optional) สร้าง GPG key → เพิ่ม `GPG_FINGERPRINT`
- [ ] Push tag เช่น `v0.1.0` เพื่อทดสอบ

---

## 📚 ลิงก์ที่เกี่ยวข้อง

- [GoReleaser Documentation](https://goreleaser.com/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Homebrew Formula Cookbook](https://docs.brew.sh/Formula-Cookbook)
- [Scoop Wiki](https://github.com/ScoopInstaller/Scoop/wiki)
- [Chocolatey Packaging](https://docs.chocolatey.org/en-us/create/create-packages)
- [WinGet Packages](https://github.com/microsoft/winget-pkgs)
- [GitHub Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)

---

**หากทำตาม checklist ครบ ระบบจะทำงานได้สมบูรณ์! 🎉**
