import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { chromium } from 'playwright';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
import { selectSizedFile } from './upload-test-files.mjs';

const root=fileURLToPath(new URL('../',import.meta.url));
let server,browser,origin;
before(async()=>{
 server=await createServer({root,logLevel:'error',define:{'import.meta.env.VITE_API_BASE':JSON.stringify('/api/v1')},server:{host:'127.0.0.1',port:0}});
 await server.listen();origin='http://127.0.0.1:'+server.httpServer.address().port;
 browser=await chromium.launch({headless:true});
});
after(async()=>{try{await browser?.close()}finally{await server?.close()}});

test('资源上传大小边界：三种图片50MB，其它类型20MB，拒绝超限和零字节',async t=>{
 const context=await browser.newContext();t.after(()=>context.close());
 await context.addInitScript(()=>localStorage.setItem('rendering-cms-token','test'));
 const uploads=[];
 await context.route('**/api/v1/**',route=>{
  const request=route.request();
  if(request.method()==='POST'){uploads.push(request.postDataJSON());return route.fulfill({status:400,json:{error:'已通过本地校验'}})}
  return route.fulfill({json:[]});
 });
 const page=await context.newPage();page.setDefaultTimeout(5000);
 await page.goto(origin+'/admin/assets',{waitUntil:'networkidle'});
 const input=page.locator('input[type=file]');
 for(const type of ['image/png','image/jpeg','image/webp','application/pdf','text/plain','application/zip']){
  const image=type.startsWith('image/'),limit=(image?50:20)*1024*1024;
  for(const size of [limit-1,limit]){
   const response=page.waitForResponse('**/admin/assets/upload-url');
   await selectSizedFile(input,`accepted-${type.replace('/','-')}-${size}`,type,size);
   await response;
   await page.getByText('已通过本地校验',{exact:true}).last().waitFor();
   assert.equal(uploads.at(-1).byteSize,size);assert.equal(uploads.at(-1).contentType,type);
  }
  const count=uploads.length;
  for(const size of [limit+1,0]){
   await selectSizedFile(input,`rejected-${type.replace('/','-')}-${size}`,type,size);
   await page.getByText(`${image?'图片':'文件'}大小必须大于 0 且不超过 ${image?50:20}MB`,{exact:true}).last().waitFor();
   assert.equal(uploads.length,count);
  }
 }
});
