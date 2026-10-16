export async function printCopy(client, saleID, print) {
  await client.post(`/sales/${saleID}/reprints`, {});
  print();
}
