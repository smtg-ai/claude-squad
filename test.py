import json
from datetime import datetime, timedelta

class OrderProcessor:
    def __init__(self):
        self.processed_orders = []
        self.inventory = {}
        self.user_discounts = {}
        
    def load_inventory(self, inventory_data):
        """Load inventory from external data"""
        for item in inventory_data:
            self.inventory[item['sku']] = {
                'quantity': item['qty'],
                'price': item['price'],
                'category': item.get('category', 'general')
            }
    
    def apply_user_discount(self, user_id, discount_percent):
        """Apply discount for a user"""
        self.user_discounts[user_id] = discount_percent / 100
    
    def validate_order_items(self, order_items):
        """Check if all items are available in sufficient quantity"""
        for item in order_items:
            sku = item['sku']
            requested_qty = item['quantity']
            
            if sku not in self.inventory:
                return False, f"Item {sku} not found"
            
            if self.inventory[sku]['quantity'] < requested_qty:
                return False, f"Insufficient stock for {sku}"
        
        return True, "Valid"
    
    def calculate_order_total(self, order_items, user_id=None):
        """Calculate total price with discounts"""
        subtotal = 0
        
        for item in order_items:
            sku = item['sku']
            quantity = item['quantity']
            price = self.inventory[sku]['price']
            subtotal += price * quantity
        
        # Apply user discount
        if user_id and user_id in self.user_discounts:
            discount = self.user_discounts[user_id]
            subtotal = subtotal * (1 - discount)
        
        # Apply category-based bulk discount
        category_totals = {}
        for item in order_items:
            sku = item['sku']
            category = self.inventory[sku]['category']
            item_total = self.inventory[sku]['price'] * item['quantity']
            
            if category in category_totals:
                category_totals[category] += item_total
            else:
                category_totals[category] = item_total
        
        # 10% additional discount if any category total > $500
        bulk_discount = 0
        for category, total in category_totals.items():
            if total > 500:
                bulk_discount += total * 0.1
        
        final_total = subtotal - bulk_discount
        return max(final_total, 0)  # Don't allow negative totals
    
    def process_order(self, order):
        """Process a single order"""
        order_id = order.get('id', f"order_{len(self.processed_orders)}")
        user_id = order.get('user_id')
        order_items = order['items']
        timestamp = datetime.now()
        
        # Validate order
        is_valid, message = self.validate_order_items(order_items)
        if not is_valid:
            return {
                'order_id': order_id,
                'status': 'failed',
                'error': message,
                'timestamp': timestamp.isoformat()
            }
        
        # Calculate total
        total = self.calculate_order_total(order_items, user_id)
        
        # Update inventory
        for item in order_items:
            sku = item['sku']
            self.inventory[sku]['quantity'] -= item['quantity']
        
        # Store processed order
        processed_order = {
            'order_id': order_id,
            'user_id': user_id,
            'items': order_items,
            'total': total,
            'status': 'completed',
            'timestamp': timestamp.isoformat()
        }
        
        self.processed_orders.append(processed_order)
        return processed_order
    
    def get_sales_report(self, start_date=None, end_date=None):
        """Generate sales report for date range"""
        if not start_date:
            start_date = datetime.now() - timedelta(days=30)
        if not end_date:
            end_date = datetime.now()
        
        filtered_orders = []
        for order in self.processed_orders:
            order_date = datetime.fromisoformat(order['timestamp'])
            if start_date <= order_date <= end_date:
                filtered_orders.append(order)
        
        # Calculate metrics
        total_revenue = sum(order['total'] for order in filtered_orders)
        total_orders = len(filtered_orders)
        
        # Top selling items
        item_sales = {}
        for order in filtered_orders:
            for item in order['items']:
                sku = item['sku']
                if sku in item_sales:
                    item_sales[sku] += item['quantity']
                else:
                    item_sales[sku] = item['quantity']
        
        # Sort by quantity sold
        top_items = []
        for sku, qty in item_sales.items():
            top_items.append((sku, qty))
        
        # Bubble sort (intentionally inefficient for this example)
        n = len(top_items)
        for i in range(n):
            for j in range(0, n - i - 1):
                if top_items[j][1] < top_items[j + 1][1]:
                    top_items[j], top_items[j + 1] = top_items[j + 1], top_items[j]
        
        return {
            'date_range': f"{start_date.date()} to {end_date.date()}",
            'total_revenue': total_revenue,
            'total_orders': total_orders,
            'average_order_value': total_revenue / total_orders if total_orders > 0 else 0,
            'top_selling_items': top_items[:10]
        }
