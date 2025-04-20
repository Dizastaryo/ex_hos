// lib/screens/my_cart_screen.dart
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/product_service.dart';
import '../services/order_service.dart';
import 'products_screen.dart';
import 'orders_screen.dart';

class MyCartScreen extends StatefulWidget {
  const MyCartScreen({super.key});

  @override
  State<MyCartScreen> createState() => _MyCartScreenState();
}

class _MyCartScreenState extends State<MyCartScreen> {
  late ProductService productService;
  late OrderService orderService;
  bool isLoading = true;
  Map<String, dynamic>? cartData;

  @override
  void initState() {
    super.initState();
    productService = Provider.of<ProductService>(context, listen: false);
    orderService = Provider.of<OrderService>(context, listen: false);
    _loadCart();
  }

  Future<void> _loadCart() async {
    setState(() => isLoading = true);
    try {
      final data = await productService.getCart();
      setState(() {
        cartData = data;
        isLoading = false;
      });
    } catch (e) {
      setState(() {
        cartData = null;
        isLoading = false;
      });
    }
  }

  Future<void> _removeItem(int productId) async {
    await productService.removeFromCart(productId);
    await _loadCart();
  }

  Future<void> _clearCart() async {
    await productService.clearCart();
    await _loadCart();
  }

  void _buySingleItem(Map<String, dynamic> item) {
    final items = [
      {
        'product_id': item['product']['id'] as int, // Явное приведение типа
        'quantity': item['quantity'] as int
      }
    ];
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (context) => OrdersScreen(items: items),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Моя корзина'),
        actions: [
          IconButton(
            icon: const Icon(Icons.delete_outline),
            onPressed: _clearCart,
          ),
        ],
      ),
      body: isLoading
          ? const Center(child: CircularProgressIndicator())
          : cartData == null || cartData!['items'].isEmpty
              ? Center(
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      const Text(
                        'Корзина пуста',
                        style: TextStyle(fontSize: 18),
                      ),
                      const SizedBox(height: 20),
                      ElevatedButton(
                        onPressed: () {
                          Navigator.pushReplacement(
                            context,
                            MaterialPageRoute(
                                builder: (context) => const ProductsScreen()),
                          );
                        },
                        child: const Text('Найти товары'),
                      ),
                    ],
                  ),
                )
              : Column(
                  children: [
                    Expanded(
                      child: ListView.builder(
                        itemCount: cartData!['items'].length,
                        itemBuilder: (context, index) {
                          final item = cartData!['items'][index];
                          return ListTile(
                            leading: const Icon(Icons.shopping_bag),
                            title: Text(item['product']['name']),
                            subtitle: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text('Количество: ${item['quantity']}'),
                                Text(
                                  'Сумма: ${(item['product']['price'] * item['quantity']).toStringAsFixed(2)} ₸',
                                ),
                              ],
                            ),
                            trailing: Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                IconButton(
                                  icon: const Icon(Icons.shopping_cart_checkout,
                                      color: Colors.green),
                                  onPressed: () => _buySingleItem(item),
                                ),
                                IconButton(
                                  icon: const Icon(Icons.delete,
                                      color: Colors.red),
                                  onPressed: () =>
                                      _removeItem(item['product']['id']),
                                ),
                              ],
                            ),
                          );
                        },
                      ),
                    ),
                    Padding(
                      padding: const EdgeInsets.all(16.0),
                      child: Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          const Text(
                            'Общий итог:',
                            style: TextStyle(
                              fontSize: 18,
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                          Text(
                            '${cartData!['total_price']} ₸',
                            style: const TextStyle(
                              fontSize: 18,
                              fontWeight: FontWeight.bold,
                              color: Colors.green,
                            ),
                          ),
                        ],
                      ),
                    )
                  ],
                ),
    );
  }
}
