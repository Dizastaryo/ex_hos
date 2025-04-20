import 'package:flutter/material.dart';
import 'package:dio/dio.dart';
import 'package:provider/provider.dart';

// Импортируй ProductsScreen
import 'products_screen.dart'; // Убедись, что путь корректный

class MyCartScreen extends StatefulWidget {
  const MyCartScreen({super.key});

  @override
  State<MyCartScreen> createState() => _MyCartScreenState();
}

class _MyCartScreenState extends State<MyCartScreen> {
  late Dio dio;
  bool isLoading = true;
  Map<String, dynamic>? cartData;

  @override
  void initState() {
    super.initState();
    dio = Provider.of<Dio>(context, listen: false);
    _loadCart();
  }

  Future<void> _loadCart() async {
    setState(() => isLoading = true);
    try {
      final response = await dio.get('http://172.20.10.2:8000/cart/');
      setState(() {
        cartData = response.data;
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
    await dio.delete('http://172.20.10.2:8000/cart/remove/$productId');
    await _loadCart();
  }

  Future<void> _clearCart() async {
    await dio.delete('http://172.20.10.2:8000/cart/clear');
    await _loadCart();
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
                            subtitle: Text(
                              '${item['quantity']} x ${item['product']['price']} ₸',
                            ),
                            trailing: IconButton(
                              icon: const Icon(Icons.delete),
                              onPressed: () =>
                                  _removeItem(item['product']['id']),
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
                            'Итого:',
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
