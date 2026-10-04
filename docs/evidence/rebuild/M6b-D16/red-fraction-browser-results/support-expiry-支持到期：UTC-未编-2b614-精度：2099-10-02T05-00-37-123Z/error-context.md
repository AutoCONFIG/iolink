# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: support-expiry.spec.ts >> 支持到期：UTC >> 未编辑到期时间的重复保存保留 UTC 精度：2099-10-02T05:00:37.123Z
- Location: e2e/support-expiry.spec.ts:8:5

# Error details

```
Error: expect(received).toEqual(expected) // deep equality

- Expected  - 1
+ Received  + 1

@@ -1,9 +1,9 @@
  Array [
    Object {
      "active": true,
-     "expiresAt": "2099-10-02T05:00:37.123Z",
+     "expiresAt": "2099-10-02T05:00:37Z",
      "name": "秒数测试成员",
      "role": "support",
      "tenantId": 1,
      "userId": 2,
    },
```

# Page snapshot

```yaml
- generic [ref=f1e1]:
  - generic [ref=f1e3]:
    - complementary [ref=f1e4]:
      - generic [ref=f1e10]:
        - strong [ref=f1e11]: IoLink
        - text: 智慧水产管理平台
      - navigation [ref=f1e12]:
        - link "运营总览" [ref=f1e13] [cursor=pointer]:
          - /url: /dashboard
        - link "养殖场与池塘" [ref=f1e18] [cursor=pointer]:
          - /url: /ponds
        - link "设备管理" [ref=f1e24] [cursor=pointer]:
          - /url: /devices
        - link "产品与物模型" [ref=f1e29] [cursor=pointer]:
          - /url: /products
        - link "报警规则" [ref=f1e34] [cursor=pointer]:
          - /url: /alarms/rules
        - link "报警中心" [ref=f1e39] [cursor=pointer]:
          - /url: /alarms
        - link "系统设置" [ref=f1e46] [cursor=pointer]:
          - /url: /system
        - link "组织与成员" [ref=f1e51] [cursor=pointer]:
          - /url: /tenants
      - generic [ref=f1e58]: 服务状态待后端健康检查
    - generic [ref=f1e61]:
      - generic [ref=f1e62]:
        - generic [ref=f1e63]:
          - button "收起导航" [expanded] [ref=f1e64] [cursor=pointer]
          - generic [ref=f1e68]:
            - heading "组织与成员" [level=1] [ref=f1e69]
            - paragraph [ref=f1e70]: 监测水质、设备与报警状态
        - generic [ref=f1e71]:
          - generic [ref=f1e72]: 演示数据
          - generic [ref=f1e74]: 同源 API · /admin/v1
          - button "管 管理员 ⌄" [ref=f1e76] [cursor=pointer]:
            - generic [ref=f1e77]: 管
            - generic [ref=f1e78]: 管理员
            - generic [ref=f1e79]: ⌄
      - main [ref=f1e80]:
        - generic [ref=f1e81]:
          - generic [ref=f1e83]:
            - heading "组织与成员" [level=2] [ref=f1e84]
            - paragraph [ref=f1e85]: 切换组织并维护成员角色、启停状态和支持授权期限。
          - generic [ref=f1e86]:
            - generic [ref=f1e87]:
              - heading "组织" [level=3] [ref=f1e89]
              - 'button "演示组织 #1 已启用" [ref=f1e91] [cursor=pointer]':
                - generic [ref=f1e92]:
                  - strong [ref=f1e93]: 演示组织
                  - generic [ref=f1e94]: "#1"
                - generic [ref=f1e95]: 已启用
            - generic [ref=f1e97]:
              - generic [ref=f1e99]:
                - heading "演示组织" [level=3] [ref=f1e100]
                - text: 角色变更会使旧权限版本失效
              - generic [ref=f1e103]:
                - table [ref=f1e105]:
                  - rowgroup [ref=f1e112]:
                    - row [ref=f1e113]:
                      - columnheader "成员" [ref=f1e114]
                      - columnheader "角色" [ref=f1e116]
                      - columnheader "支持到期（上海时间）" [ref=f1e118]
                      - columnheader "状态" [ref=f1e120]
                      - columnheader "操作" [ref=f1e122]
                - generic [ref=f1e125]:
                  - table [ref=f1e128]:
                    - rowgroup [ref=f1e135]:
                      - row [ref=f1e136]:
                        - cell "秒数测试成员" [ref=f1e137]
                        - cell "support" [ref=f1e139]:
                          - generic [ref=f1e142] [cursor=pointer]:
                            - generic:
                              - combobox [ref=f1e144]
                              - generic [ref=f1e145]: support
                        - cell [ref=f1e150]:
                          - combobox "支持角色必填" [ref=f1e160]: 2099-10-02 13:00:37
                        - cell [ref=f1e161]:
                          - generic [ref=f1e163]:
                            - switch [checked]
                            - generic [ref=f1e164] [cursor=pointer]
                        - cell [ref=f1e166]:
                          - button "保存" [active] [ref=f1e168] [cursor=pointer]
                  - generic [ref=f1e171] [cursor=pointer]
  - alert [ref=f1e172]:
    - paragraph [ref=f1e176]: 成员权限已更新
```

# Test source

```ts
  1  | import { expect, test } from '@playwright/test'
  2  | import { writeFile } from 'node:fs/promises'
  3  | 
  4  | for (const timezoneId of ['Asia/Shanghai', 'UTC', 'America/New_York']) {
  5  |   test.describe(`支持到期：${timezoneId}`, () => {
  6  |     test.use({ timezoneId })
  7  |     for (const expiresAt of ['2099-10-02T05:00:37Z', '2099-10-02T05:00:37.123Z']) {
  8  |     test(`未编辑到期时间的重复保存保留 UTC 精度：${expiresAt}`, async ({ page }, testInfo) => {
  9  |       await page.addInitScript((expiry) => {
  10 |         if (!localStorage.getItem('iolink.demo.tenant-members')) {
  11 |           localStorage.setItem('iolink.demo.tenant-members', JSON.stringify([
  12 |             { tenantId: 1, userId: 2, name: '秒数测试成员', role: 'support', active: true, expiresAt: expiry },
  13 |           ]))
  14 |         }
  15 |       }, expiresAt)
  16 |       await page.goto('/login')
  17 |       await page.getByRole('button', { name: '进入控制台' }).click()
  18 |       await expect(page).toHaveURL(/\/dashboard$/)
  19 |       await page.goto('/tenants')
  20 |       const member = page.getByRole('row', { name: /秒数测试成员/ })
  21 |       await expect(member).toBeVisible()
  22 |       for (let save = 1; save <= 2; save++) {
  23 |         await member.getByRole('button', { name: '保存' }).click()
  24 |         await expect(page.getByText('成员权限已更新')).toBeVisible()
  25 |         const persisted = await page.evaluate(() => JSON.parse(localStorage.getItem('iolink.demo.tenant-members') ?? 'null'))
  26 |         const artifact = testInfo.outputPath(`unchanged-save-${save}.json`)
  27 |         await writeFile(artifact, JSON.stringify(persisted, null, 2))
  28 |         await testInfo.attach(`unchanged-save-${save}`, { path: artifact, contentType: 'application/json' })
> 29 |         expect(persisted).toEqual([
     |                           ^ Error: expect(received).toEqual(expected) // deep equality
  30 |           { tenantId: 1, userId: 2, name: '秒数测试成员', role: 'support', active: true, expiresAt },
  31 |         ])
  32 |         await page.reload()
  33 |         await expect(member.locator('input[placeholder="支持角色必填"]')).toHaveValue('2099-10-02 13:00:37')
  34 |       }
  35 |       for (const width of [375, 768, 1280]) {
  36 |         await page.setViewportSize({ width, height: 960 })
  37 |         await member.locator('input[placeholder="支持角色必填"]').scrollIntoViewIfNeeded()
  38 |         await page.screenshot({ path: testInfo.outputPath(`unchanged-${width}.png`), fullPage: true })
  39 |       }
  40 |     })
  41 |     }
  42 | 
  43 |     test('上海时间显示、编辑、保存和重载保持同一瞬间', async ({ page }, testInfo) => {
  44 |       await page.addInitScript(() => {
  45 |         if (!localStorage.getItem('iolink.demo.tenant-members')) {
  46 |           localStorage.setItem('iolink.demo.tenant-members', JSON.stringify([
  47 |             { tenantId: 1, userId: 2, name: '时区测试成员', role: 'support', active: true, expiresAt: '2099-10-02T05:00:37Z' },
  48 |           ]))
  49 |         }
  50 |       })
  51 |       await page.goto('/login')
  52 |       await page.getByRole('button', { name: '进入控制台' }).click()
  53 |       await expect(page).toHaveURL(/\/dashboard$/)
  54 |       await page.goto('/tenants')
  55 |       const member = page.getByRole('row', { name: /时区测试成员/ })
  56 |       await expect(member).toBeVisible()
  57 |       const expiry = member.locator('input[placeholder="支持角色必填"]')
  58 |       await expect(expiry).toHaveValue('2099-10-02 13:00:37')
  59 |       await expiry.fill('2099-10-03 13:00:47')
  60 |       await expiry.press('Enter')
  61 |       await expiry.blur()
  62 |       await expect(expiry).toHaveValue('2099-10-03 13:00:47')
  63 |       await member.getByRole('button', { name: '保存' }).click()
  64 |       await expect(page.getByText('成员权限已更新')).toBeVisible()
  65 |       const persisted = await page.evaluate(() => JSON.parse(localStorage.getItem('iolink.demo.tenant-members') ?? 'null'))
  66 |       const artifact = testInfo.outputPath('edited-save.json')
  67 |       await writeFile(artifact, JSON.stringify(persisted, null, 2))
  68 |       await testInfo.attach('edited-save', { path: artifact, contentType: 'application/json' })
  69 |       expect(persisted).toEqual([
  70 |         { tenantId: 1, userId: 2, name: '时区测试成员', role: 'support', active: true, expiresAt: '2099-10-03T05:00:47Z' },
  71 |       ])
  72 |       await page.reload()
  73 |       await expect(page.getByRole('row', { name: /时区测试成员/ }).locator('input').nth(1)).toHaveValue('2099-10-03 13:00:47')
  74 |       await page.screenshot({ path: testInfo.outputPath('edited-reloaded.png'), fullPage: true })
  75 |     })
  76 |   })
  77 | }
  78 | 
```