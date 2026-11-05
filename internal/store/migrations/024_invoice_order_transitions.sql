-- Migration 022 introduced invoice_approved in the table constraint but the
-- earlier transition trigger remained in force on already-running databases.
CREATE OR REPLACE FUNCTION enforce_order_status_transition() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status = NEW.status
    OR (OLD.status = 'pending_payment' AND NEW.status IN ('paid', 'invoice_approved', 'cancelled'))
    OR (OLD.status = 'invoice_approved' AND NEW.status IN ('fulfilled', 'cancelled'))
    OR (OLD.status = 'paid' AND NEW.status IN ('fulfilled', 'cancelled'))
    OR (OLD.status = 'fulfilled' AND NEW.status = 'paid')
    OR (OLD.status = 'cancelled' AND NEW.status = 'paid') THEN
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'invalid order status transition: % -> %', OLD.status, NEW.status USING ERRCODE = '23514';
END; $$;
