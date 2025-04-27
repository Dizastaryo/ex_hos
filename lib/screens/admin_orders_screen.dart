import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/order_service.dart';

class AdminOrdersScreen extends StatefulWidget {
  const AdminOrdersScreen({Key? key}) : super(key: key);

  @override
  State<AdminOrdersScreen> createState() => _AdminOrdersScreenState();
}

class _AdminOrdersScreenState extends State<AdminOrdersScreen> {
  late final OrderService orderService;
  bool isLoading = true;
  List<dynamic> orders = [];
  List<String> statuses = [];

  @override
  void initState() {
    super.initState();
    orderService = Provider.of<OrderService>(context, listen: false);
    _loadData();
  }

  Future<void> _loadData() async {
    setState(() => isLoading = true);
    try {
      final results = await Future.wait([
        orderService.getAllOrders(),
        orderService.getOrderStatuses(),
      ]);
      orders = results[0] as List<dynamic>;
      statuses = results[1] as List<String>;
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка загрузки данных: $e')),
      );
    } finally {
      setState(() => isLoading = false);
    }
  }

  void _showOrderDetails(Map<String, dynamic> order) {
    showDialog(
      context: context,
      builder: (_) => AlertDialog(
        title: Text('Заказ #${order['id']}'),
        content: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Пользователь: ${order['user_id']}'),
              const SizedBox(height: 8),
              Text('Адрес: ${order['shipping_address']}'),
              const SizedBox(height: 8),
              Text('Статус: ${order['status']}'),
              const SizedBox(height: 8),
              Text('Товары:'),
              ...List<Widget>.from(
                (order['items'] as List<dynamic>).map(
                  (item) => Text(
                    '- ID: ${item['product_id']}  × ${item['quantity']}',
                  ),
                ),
              ),
              const SizedBox(height: 8),
              Text('Итог: ${order['total']}₸'),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Закрыть'),
          ),
        ],
      ),
    );
  }

  Future<void> _changeOrderStatus(int orderId, String newStatus) async {
    try {
      await orderService.updateOrderStatus(orderId, newStatus);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Статус заказа #$orderId изменён на $newStatus'),
        ),
      );
      _loadData();
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка при изменении статуса: $e')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Все заказы')),
      body: isLoading
          ? const Center(child: CircularProgressIndicator())
          : orders.isEmpty
              ? const Center(child: Text('Нет заказов'))
              : ListView.builder(
                  itemCount: orders.length,
                  itemBuilder: (_, i) {
                    final order = orders[i] as Map<String, dynamic>;
                    return Card(
                      margin: const EdgeInsets.symmetric(
                        horizontal: 12,
                        vertical: 6,
                      ),
                      child: ListTile(
                        title: Text('Заказ #${order['id']}'),
                        subtitle: Text('Статус: ${order['status']}'),
                        trailing: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            IconButton(
                              icon: const Icon(Icons.info_outline),
                              onPressed: () => _showOrderDetails(order),
                            ),
                            PopupMenuButton<String>(
                              icon: const Icon(Icons.edit),
                              onSelected: (status) => _changeOrderStatus(
                                order['id'] as int,
                                status,
                              ),
                              itemBuilder: (_) => statuses
                                  .map(
                                    (s) => PopupMenuItem(
                                      value: s,
                                      child: Text(s),
                                    ),
                                  )
                                  .toList(),
                            ),
                          ],
                        ),
                      ),
                    );
                  },
                ),
    );
  }
}
