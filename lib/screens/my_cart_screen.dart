import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/product_service.dart';
import '../services/order_service.dart';
import 'products_screen.dart';
import 'orders_screen.dart';
import 'product_detail_screen.dart';

class MyCartScreen extends StatefulWidget {
  const MyCartScreen({Key? key}) : super(key: key);

  @override
  State<MyCartScreen> createState() => _MyCartScreenState();
}

class _MyCartScreenState extends State<MyCartScreen> {
  late ProductService _productService;
  late OrderService _orderService;
  bool _isLoading = true;
  List<Map<String, dynamic>> _cartItems = [];
  double _totalPrice = 0.0;

  @override
  void initState() {
    super.initState();
    _productService = Provider.of<ProductService>(context, listen: false);
    _orderService = Provider.of<OrderService>(context, listen: false);
    _loadCart();
  }

  Future<void> _loadCart() async {
    setState(() => _isLoading = true);
    try {
      final response = await _productService.getCart();
      final rawItems = response['items'] as List<dynamic>? ?? [];
      final parsedItems =
          rawItems.map((e) => Map<String, dynamic>.from(e as Map)).toList();
      setState(() {
        _cartItems = parsedItems;
        _totalPrice = (response['total_price'] as num?)?.toDouble() ?? 0.0;
      });
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка загрузки корзины: \$e')),
      );
    } finally {
      setState(() => _isLoading = false);
    }
  }

  Future<void> _updateQuantity(int productId, int newQty) async {
    if (newQty < 1) return;
    setState(() => _isLoading = true);
    try {
      await _productService.updateCart(productId, newQty);
      await _loadCart();
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка обновления количества: \$e')),
      );
    } finally {
      setState(() => _isLoading = false);
    }
  }

  Future<void> _removeItem(int productId) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (_) => AlertDialog(
        title: const Text('Удалить товар'),
        content: const Text('Вы уверены, что хотите удалить товар из корзины?'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(context, false),
              child: const Text('Отмена')),
          TextButton(
              onPressed: () => Navigator.pop(context, true),
              child: const Text('Удалить')),
        ],
      ),
    );
    if (confirm != true) return;

    setState(() => _isLoading = true);
    try {
      await _productService.removeFromCart(productId);
      await _loadCart();
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка удаления товара: \$e')),
      );
    } finally {
      setState(() => _isLoading = false);
    }
  }

  Future<void> _clearCart() async {
    if (_cartItems.isEmpty) return;
    final confirm = await showDialog<bool>(
      context: context,
      builder: (_) => AlertDialog(
        title: const Text('Очистить корзину'),
        content:
            const Text('Вы уверены, что хотите удалить все товары из корзины?'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(context, false),
              child: const Text('Отмена')),
          TextButton(
              onPressed: () => Navigator.pop(context, true),
              child: const Text('Очистить')),
        ],
      ),
    );
    if (confirm != true) return;

    setState(() => _isLoading = true);
    try {
      await _productService.clearCart();
      await _loadCart();
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка очистки корзины: \$e')),
      );
    } finally {
      setState(() => _isLoading = false);
    }
  }

  void _navigateToCheckout(List<Map<String, int>> items) {
    Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => OrdersScreen(items: items)),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Моя корзина'),
        actions: [
          IconButton(
            icon: const Icon(Icons.delete_forever),
            onPressed: _cartItems.isEmpty ? null : _clearCart,
            tooltip: 'Очистить корзину',
          ),
        ],
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : _cartItems.isEmpty
              ? _buildEmptyCart()
              : Column(
                  children: [
                    Expanded(
                      child: ListView.separated(
                        itemCount: _cartItems.length,
                        separatorBuilder: (_, __) => const Divider(),
                        itemBuilder: (_, i) => _buildCartItem(_cartItems[i]),
                      ),
                    ),
                    _buildTotalSection(),
                  ],
                ),
    );
  }

  Widget _buildEmptyCart() => Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.shopping_cart_outlined,
                size: 80, color: Colors.grey),
            const SizedBox(height: 20),
            const Text('Ваша корзина пуста',
                style: TextStyle(fontSize: 18, color: Colors.grey)),
            const SizedBox(height: 20),
            ElevatedButton.icon(
              icon: const Icon(Icons.search),
              label: const Text('Перейти к товарам'),
              onPressed: () => Navigator.pushReplacement(
                context,
                MaterialPageRoute(builder: (_) => const ProductsScreen()),
              ),
            ),
          ],
        ),
      );

  Widget _buildCartItem(Map<String, dynamic> item) {
    final productData = item['product'] as Map<String, dynamic>?;
    final qty = item['quantity'] as int?;
    if (productData == null || qty == null) {
      return const ListTile(
        leading: Icon(Icons.error_outline, color: Colors.red),
        title: Text('Некорректные данные товара'),
      );
    }
    final id = productData['id'] as int;
    final name = productData['name'] as String? ?? '';
    final price = (productData['price'] as num).toDouble();
    final images = (productData['images'] as List).cast<Map<String, dynamic>>();
    final imgUrl = images.isNotEmpty
        ? 'http://172.20.10.2:8000${images.first['image_url']}'
        : 'https://via.placeholder.com/150';

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      elevation: 2,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: Image.network(imgUrl,
                  width: 60, height: 60, fit: BoxFit.cover),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: InkWell(
                onTap: () => Navigator.push(
                  context,
                  MaterialPageRoute(
                      builder: (_) => ProductDetailScreen(productId: id)),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(name,
                        style: const TextStyle(
                            fontSize: 16, fontWeight: FontWeight.bold)),
                    const SizedBox(height: 4),
                    Text('${price.toStringAsFixed(2)} ₸ x $qty'),
                    Text('Итого: ${(price * qty).toStringAsFixed(2)} ₸',
                        style: const TextStyle(fontWeight: FontWeight.bold)),
                    const SizedBox(height: 8),
                    Row(
                      children: [
                        IconButton(
                          icon: const Icon(Icons.remove_circle_outline),
                          onPressed: () => _updateQuantity(id, qty - 1),
                          constraints: const BoxConstraints(),
                          padding: EdgeInsets.zero,
                        ),
                        Text(qty.toString()),
                        IconButton(
                          icon: const Icon(Icons.add_circle_outline),
                          onPressed: () => _updateQuantity(id, qty + 1),
                          constraints: const BoxConstraints(),
                          padding: EdgeInsets.zero,
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
            Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                IconButton(
                  icon: const Icon(Icons.arrow_forward_ios),
                  onPressed: () => _navigateToCheckout([
                    {'product_id': id, 'quantity': qty}
                  ]),
                ),
                IconButton(
                  icon: const Icon(Icons.delete_outline),
                  color: Colors.red,
                  onPressed: () => _removeItem(id),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildTotalSection() {
    final items = _cartItems
        .map((e) => {
              'product_id': (e['product'] as Map<String, dynamic>)['id'] as int,
              'quantity': e['quantity'] as int
            })
        .toList();
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.grey[100],
        borderRadius: const BorderRadius.vertical(top: Radius.circular(16)),
      ),
      child: Column(
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text('Общая сумма:',
                  style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
              Text('${_totalPrice.toStringAsFixed(2)} ₸',
                  style: const TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.bold,
                      color: Colors.green)),
            ],
          ),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: () => _navigateToCheckout(items),
              child: const Text('Оформить весь заказ',
                  style: TextStyle(fontSize: 16)),
            ),
          ),
        ],
      ),
    );
  }
}
