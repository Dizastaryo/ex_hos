import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/product.dart';
import '../services/product_service.dart';
import '../services/order_service.dart';
import 'payment_screen.dart';

class OrdersScreen extends StatefulWidget {
  final List<Map<String, int>> items;
  const OrdersScreen({Key? key, required this.items}) : super(key: key);

  @override
  State<OrdersScreen> createState() => _OrdersScreenState();
}

class _OrdersScreenState extends State<OrdersScreen> {
  final _formKey = GlobalKey<FormState>();
  late final ProductService productService;
  late final OrderService orderService;

  final _streetController = TextEditingController();
  final _houseController = TextEditingController();
  final _apartmentController = TextEditingController();
  final _entranceController = TextEditingController();

  bool isLoading = false;

  @override
  void initState() {
    super.initState();
    productService = Provider.of<ProductService>(context, listen: false);
    orderService = Provider.of<OrderService>(context, listen: false);
  }

  @override
  void dispose() {
    _streetController.dispose();
    _houseController.dispose();
    _apartmentController.dispose();
    _entranceController.dispose();
    super.dispose();
  }

  Future<void> _submitOrder() async {
    if (!_formKey.currentState!.validate()) return;

    final address = '${_streetController.text.trim()}, '
        'дом ${_houseController.text.trim()}, '
        'кв. ${_apartmentController.text.trim()}, '
        'подъезд ${_entranceController.text.trim()}';

    setState(() => isLoading = true);
    try {
      final response = await orderService.createOrder(widget.items, address);
      final orderId = response['id'] as int;
      final orderTotal = (response['total'] as num).toDouble();

      Navigator.push(
        context,
        MaterialPageRoute(
          builder: (_) => PaymentScreen(
            orderId: orderId,
            orderTotal: orderTotal,
          ),
        ),
      );
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка при оформлении: $e')),
      );
    } finally {
      setState(() => isLoading = false);
    }
  }

  Widget _buildOrderSummary() {
    return Card(
      margin: const EdgeInsets.symmetric(vertical: 8),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      elevation: 2,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('Ваш заказ',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
            const SizedBox(height: 12),
            ...widget.items.map((item) {
              return FutureBuilder<Product>(
                future: productService.getProductById(item['product_id']!),
                builder: (context, snap) {
                  if (!snap.hasData) {
                    return const Padding(
                      padding: EdgeInsets.symmetric(vertical: 8),
                      child: Center(child: CircularProgressIndicator()),
                    );
                  }
                  final p = snap.data!;
                  final imageUrl = p.imageUrls.isNotEmpty
                      ? productService.getImageUrl(p.imageUrls.first)
                      : 'https://via.placeholder.com/80';

                  return Padding(
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    child: Row(
                      children: [
                        // Картинка товара
                        ClipRRect(
                          borderRadius: BorderRadius.circular(8),
                          child: Image.network(
                            imageUrl,
                            width: 80,
                            height: 80,
                            fit: BoxFit.cover,
                          ),
                        ),
                        const SizedBox(width: 12),
                        // Описание товара и количество
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(p.name,
                                  style: const TextStyle(
                                      fontSize: 16,
                                      fontWeight: FontWeight.bold)),
                              const SizedBox(height: 4),
                              Text('Кол-во: ${item['quantity']}'),
                              const SizedBox(height: 4),
                              Text(
                                'Итого: ${(p.price * item['quantity']!).toStringAsFixed(2)} ₸',
                                style: const TextStyle(
                                    fontWeight: FontWeight.w600),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  );
                },
              );
            }).toList(),
          ],
        ),
      ),
    );
  }

  Widget _buildAddressForm() {
    return Card(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      elevation: 2,
      margin: const EdgeInsets.symmetric(vertical: 8),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Form(
          key: _formKey,
          child: Column(
            children: [
              const Align(
                alignment: Alignment.centerLeft,
                child: Text('Адрес доставки',
                    style:
                        TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _streetController,
                decoration: const InputDecoration(
                  labelText: 'Улица',
                  border: OutlineInputBorder(),
                ),
                validator: (v) =>
                    v == null || v.trim().isEmpty ? 'Введите улицу' : null,
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    flex: 2,
                    child: TextFormField(
                      controller: _houseController,
                      decoration: const InputDecoration(
                        labelText: 'Дом',
                        border: OutlineInputBorder(),
                      ),
                      validator: (v) =>
                          v == null || v.trim().isEmpty ? 'Укажите дом' : null,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    flex: 2,
                    child: TextFormField(
                      controller: _apartmentController,
                      decoration: const InputDecoration(
                        labelText: 'Квартира',
                        border: OutlineInputBorder(),
                      ),
                      validator: (v) =>
                          v == null || v.trim().isEmpty ? 'Укажите кв.' : null,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    flex: 1,
                    child: TextFormField(
                      controller: _entranceController,
                      decoration: const InputDecoration(
                        labelText: 'Подъезд',
                        border: OutlineInputBorder(),
                      ),
                      validator: (v) => v == null || v.trim().isEmpty
                          ? 'Укажите подъезд'
                          : null,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Оформление заказа')),
      body: isLoading
          ? const Center(child: CircularProgressIndicator())
          : SingleChildScrollView(
              padding: const EdgeInsets.all(16),
              child: Column(
                children: [
                  _buildOrderSummary(),
                  _buildAddressForm(),
                  const SizedBox(height: 20),
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton(
                      style: ElevatedButton.styleFrom(
                        padding: const EdgeInsets.symmetric(vertical: 16),
                        shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(12)),
                      ),
                      onPressed: _submitOrder,
                      child: const Text(
                        'Перейти к оплате',
                        style: TextStyle(
                            fontSize: 16, fontWeight: FontWeight.w600),
                      ),
                    ),
                  ),
                ],
              ),
            ),
    );
  }
}
