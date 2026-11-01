export async function selectSizedFile(input, name, type, size) {
  await input.evaluate((element, { name, type, size }) => {
    const transfer = new DataTransfer();
    transfer.items.add(new File([new Uint8Array(size)], name, { type }));
    element.files = transfer.files;
    element.dispatchEvent(new Event('change', { bubbles: true }));
  }, { name, type, size });
}
