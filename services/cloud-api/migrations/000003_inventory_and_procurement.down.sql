-- 000003_inventory_and_procurement.down.sql

DROP TABLE IF EXISTS vendor_invoice_documents CASCADE;
DROP TABLE IF EXISTS vendor_invoice_lines CASCADE;
DROP TABLE IF EXISTS vendor_invoices CASCADE;
DROP TABLE IF EXISTS goods_receipt_lines CASCADE;
DROP TABLE IF EXISTS goods_receipts CASCADE;
DROP TABLE IF EXISTS po_lines CASCADE;
DROP TABLE IF EXISTS purchase_orders CASCADE;
DROP TABLE IF EXISTS supplier_items CASCADE;
DROP TABLE IF EXISTS suppliers CASCADE;
DROP TABLE IF EXISTS transfer_lines CASCADE;
DROP TABLE IF EXISTS warehouse_transfers CASCADE;
DROP TABLE IF EXISTS requisition_lines CASCADE;
DROP TABLE IF EXISTS kitchen_requisitions CASCADE;
DROP TABLE IF EXISTS stock_movements CASCADE;
DROP TABLE IF EXISTS stock_balances CASCADE;
DROP TABLE IF EXISTS stock_locations CASCADE;
DROP TABLE IF EXISTS recipe_lines CASCADE;
DROP TABLE IF EXISTS recipe_versions CASCADE;
DROP TABLE IF EXISTS unit_conversions CASCADE;
DROP TABLE IF EXISTS ingredients CASCADE;
