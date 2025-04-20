import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/order_service.dart';

class PaymentStatusScreen extends StatefulWidget {
  final int orderId;
  const PaymentStatusScreen({Key? key, required this.orderId})
      : super(key: key);

  @override
  State<PaymentStatusScreen> createState() => _PaymentStatusScreenState();
}

class _PaymentStatusScreenState extends State<PaymentStatusScreen> {
  String paymentStatus = "Загрузка...";
  bool isLoading = false;

  Future<void> _getPaymentStatus() async {
    setState(() => isLoading = true);
    try {
      final response = await Provider.of<OrderService>(context, listen: false)
          .getPaymentStatus(widget.orderId);
      setState(() {
        paymentStatus = response['status'];
      });
    } catch (e) {
      setState(() {
        paymentStatus = "Ошибка получения статуса";
      });
    } finally {
      setState(() => isLoading = false);
    }
  }

  @override
  void initState() {
    super.initState();
    _getPaymentStatus();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(
        title: const Text('Статус платежа'),
        centerTitle: true,
        backgroundColor: Colors.blue.shade700,
      ),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Card(
            elevation: 6,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(16),
            ),
            child: Padding(
              padding: const EdgeInsets.symmetric(vertical: 32, horizontal: 24),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  _buildStatusIcon(),
                  const SizedBox(height: 24),
                  Text(
                    'Статус платежа:',
                    style: theme.textTheme.titleMedium,
                  ),
                  const SizedBox(height: 12),
                  AnimatedContainer(
                    duration: const Duration(milliseconds: 300),
                    padding: const EdgeInsets.symmetric(
                        vertical: 14, horizontal: 20),
                    decoration: BoxDecoration(
                      color: _getStatusColor().withOpacity(0.9),
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Text(
                      paymentStatus,
                      style: const TextStyle(
                        fontSize: 20,
                        color: Colors.white,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  const SizedBox(height: 32),
                  isLoading
                      ? const CircularProgressIndicator()
                      : ElevatedButton.icon(
                          icon: const Icon(Icons.refresh),
                          label: const Text('Обновить статус'),
                          style: ElevatedButton.styleFrom(
                            backgroundColor: Colors.blue.shade600,
                            minimumSize: const Size.fromHeight(50),
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(12),
                            ),
                          ),
                          onPressed: _getPaymentStatus,
                        ),
                  const SizedBox(height: 16),
                  TextButton(
                    onPressed: () => Navigator.pushNamedAndRemoveUntil(
                        context, '/main', (route) => false),
                    child: const Text('Вернуться на главную'),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildStatusIcon() {
    IconData icon;
    Color color;

    switch (paymentStatus.toLowerCase()) {
      case 'успешно':
        icon = Icons.check_circle_outline;
        color = Colors.green;
        break;
      case 'в обработке':
        icon = Icons.hourglass_bottom;
        color = Colors.orange;
        break;
      case 'ошибка':
        icon = Icons.error_outline;
        color = Colors.red;
        break;
      default:
        icon = Icons.sync;
        color = Colors.grey;
    }

    return Icon(icon, size: 80, color: color);
  }

  Color _getStatusColor() {
    switch (paymentStatus.toLowerCase()) {
      case 'успешно':
        return Colors.green;
      case 'в обработке':
        return Colors.orange;
      case 'ошибка':
        return Colors.red;
      default:
        return Colors.grey;
    }
  }
}
