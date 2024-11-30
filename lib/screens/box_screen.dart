import 'package:flutter/material.dart';
import 'package:model_viewer_plus/model_viewer_plus.dart';
import '../models/box_model.dart';
import 'payment_screen.dart'; // Экран оплаты

class BoxScreen extends StatefulWidget {
  final BoxModel boxModel;

  const BoxScreen({super.key, required this.boxModel});

  @override
  State<BoxScreen> createState() => _BoxScreenState();
}

class _BoxScreenState extends State<BoxScreen> {
  int? selectedTariff; // Выбранный тариф
  bool courierNeeded = false; // Флаг для курьера

  // Тарифы на основе размера коробки
  List<Map<String, dynamic>> getTariffs(BoxType type) {
    switch (type) {
      case BoxType.xxsmall:
        return [
          {'months': 1, 'price': 8000},
          {'months': 2, 'price': 15000},
          {'months': 3, 'price': 21600},
        ];
      case BoxType.xsmall:
        return [
          {'months': 1, 'price': 3500},
          {'months': 2, 'price': 6500},
          {'months': 3, 'price': 9450},
        ];
      case BoxType.small:
        return [
          {'months': 1, 'price': 10000},
          {'months': 2, 'price': 19000},
          {'months': 3, 'price': 27000},
        ];
      case BoxType.medium:
        return [
          {'months': 1, 'price': 13000},
          {'months': 2, 'price': 24000},
          {'months': 3, 'price': 34200},
        ];
      case BoxType.large:
        return [
          {'months': 1, 'price': 15000},
          {'months': 2, 'price': 28000},
          {'months': 3, 'price': 40500},
        ];
      case BoxType.xlarge:
        return [
          {'months': 1, 'price': 30000},
          {'months': 2, 'price': 56000},
          {'months': 3, 'price': 81000},
        ];
      default:
        return [];
    }
  }

  @override
  Widget build(BuildContext context) {
    final tariffs = getTariffs(widget.boxModel.type);

    return Scaffold(
      appBar: AppBar(
        title: Text('Бокс ${widget.boxModel.id}'),
        backgroundColor: const Color(0xFF6C9942),
        elevation: 0,
      ),
      body: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 3D Модель
            Padding(
              padding: const EdgeInsets.all(16.0),
              child: Container(
                height: 300,
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(16),
                  boxShadow: [
                    BoxShadow(
                      color: Colors.black26,
                      blurRadius: 10,
                      offset: Offset(0, 4),
                    ),
                  ],
                ),
                child: ModelViewer(
                  src: "assets/3d_models/${widget.boxModel.id}.glb",
                  autoRotate: true,
                  cameraControls: true,
                  alt: "3D модель ${widget.boxModel.id}",
                ),
              ),
            ),

            // Информация о тарифах
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16.0),
              child: Text(
                'Тарифы ${widget.boxModel.id}:',
                style: TextStyle(
                  fontSize: 24,
                  fontWeight: FontWeight.bold,
                  color: Color(0xFF6C9942),
                ),
              ),
            ),
            const SizedBox(height: 8),

            // Список тарифов
            AnimatedSwitcher(
              duration: const Duration(milliseconds: 300),
              child: ListView.builder(
                shrinkWrap: true,
                physics: const NeverScrollableScrollPhysics(),
                itemCount: tariffs.length,
                itemBuilder: (context, index) {
                  final tariff = tariffs[index];
                  return Card(
                    margin: const EdgeInsets.symmetric(
                        vertical: 8.0, horizontal: 16.0),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(16),
                    ),
                    elevation: 5,
                    shadowColor: Colors.black26,
                    child: Material(
                      color: Colors.transparent,
                      child: InkWell(
                        borderRadius: BorderRadius.circular(16),
                        onTap: () {
                          setState(() {
                            selectedTariff = index;
                          });
                        },
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                              vertical: 16.0, horizontal: 20.0),
                          decoration: BoxDecoration(
                            color: selectedTariff == index
                                ? Color(0xFF6C9942).withOpacity(0.2)
                                : Colors.transparent,
                            borderRadius: BorderRadius.circular(16),
                          ),
                          child: ListTile(
                            title: Text(
                              '${tariff['months']} месяц(ев): ${tariff['price']} тг',
                              style: TextStyle(
                                  fontSize: 16, fontWeight: FontWeight.w600),
                            ),
                            leading: Radio<int>(
                              value: index,
                              groupValue: selectedTariff,
                              onChanged: (value) {
                                setState(() {
                                  selectedTariff = value;
                                });
                              },
                            ),
                          ),
                        ),
                      ),
                    ),
                  );
                },
              ),
            ),

            // Опция "Курьер нужен"
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16.0),
              child: Row(
                children: [
                  Transform.scale(
                    scale: 1.2,
                    child: Switch(
                      value: courierNeeded,
                      onChanged: (value) {
                        setState(() {
                          courierNeeded = value;
                        });
                      },
                      activeColor: Color(0xFF6C9942),
                    ),
                  ),
                  const SizedBox(width: 8),
                  const Text(
                    'Курьер нужен',
                    style: TextStyle(fontSize: 16),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),

            // Кнопка "Оплатить"
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16.0),
              child: ElevatedButton(
                onPressed: selectedTariff != null
                    ? () {
                        final selectedPlan = tariffs[selectedTariff!];
                        Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder: (context) => PaymentScreen(
                              boxId: widget.boxModel.id,
                              months: selectedPlan['months'],
                              price: selectedPlan['price'],
                              courierNeeded: courierNeeded,
                            ),
                          ),
                        );
                      }
                    : null,
                style: ElevatedButton.styleFrom(
                  backgroundColor: selectedTariff != null
                      ? const Color(0xFF6C9942)
                      : Colors.grey,
                  padding: const EdgeInsets.symmetric(vertical: 18.0),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(16),
                  ),
                  elevation: 5,
                ),
                child: const Center(
                  child: Text(
                    'Оплатить',
                    style: TextStyle(fontSize: 18, color: Colors.white),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
