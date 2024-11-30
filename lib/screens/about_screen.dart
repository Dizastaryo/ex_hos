// lib/screens/about_screen.dart
import 'package:flutter/material.dart';
import 'package:carousel_slider/carousel_slider.dart';
import 'package:google_fonts/google_fonts.dart';
import 'package:dio/dio.dart';
import 'package:provider/provider.dart';

class AboutScreen extends StatefulWidget {
  const AboutScreen({super.key});

  @override
  _AboutScreenState createState() => _AboutScreenState();
}

class _AboutScreenState extends State<AboutScreen> {
  final List<Map<String, String>> boxData = [
    {
      'image': 'assets/box_img/XXS.png',
      'title': 'XXS Бокс',
      'description':
          'Идеален для хранения небольших предметов, таких как документы, мелкие аксессуары или игрушки.',
    },
    {
      'image': 'assets/box_img/XS.png',
      'title': 'XS Бокс',
      'description':
          'Подходит для хранения книг, косметики или небольших бытовых предметов.',
    },
    {
      'image': 'assets/box_img/S.png',
      'title': 'S Бокс',
      'description':
          'Просторный бокс для одежды, сезонных вещей или крупных бытовых предметов.',
    },
    {
      'image': 'assets/box_img/M.png',
      'title': 'M Бокс',
      'description':
          'Идеален для хранения более крупных вещей: техника, посуда или текстиль.',
    },
    {
      'image': 'assets/box_img/L.png',
      'title': 'L Бокс',
      'description':
          'Подходит для крупных и тяжёлых предметов: спортивный инвентарь, мебель или строительные материалы.',
    },
    {
      'image': 'assets/box_img/XL.png',
      'title': 'XL Бокс',
      'description':
          'Для хранения самых объемных вещей, таких как мебель или крупное оборудование.',
    },
  ];

  int _currentIndex = 0;
  double _buttonHeight = 60.0; // Начальная высота кнопки

  Future<void> fetchData() async {
    final dio = Provider.of<Dio>(context, listen: false);
    try {
      final response = await dio.get(
        'http://172.20.10.3:8080/user-api/v1/user/getWithToken',
      );
      print(response.requestOptions.headers);

      if (response.statusCode == 200) {
        print('Data fetched successfully: ${response.data}');
        // Здесь можно обработать данные и обновить состояние
      } else {
        throw Exception('Error fetching data: ${response.data}');
      }
    } catch (e) {
      print('Error fetching data: $e');
      // Вы можете показать сообщение об ошибке пользователю
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: null, // Убираем AppBar
      body: Container(
        color: const Color(0xFFDCEAD3), // Основной цвет фона
        child: Column(
          children: [
            // Слайдер с изображениями
            CarouselSlider(
              options: CarouselOptions(
                height: 240.0,
                autoPlay: true,
                enlargeCenterPage: true,
                viewportFraction: 0.75,
                onPageChanged: (index, reason) {
                  setState(() {
                    _currentIndex = index;
                  });
                },
              ),
              items: boxData.map((box) {
                return Container(
                  margin: const EdgeInsets.symmetric(horizontal: 8.0),
                  decoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(20),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black26,
                        blurRadius: 15,
                        offset: const Offset(0, 6),
                      ),
                    ],
                  ),
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(20),
                    child: Image.asset(
                      box['image']!,
                      fit: BoxFit.cover,
                    ),
                  ),
                );
              }).toList(),
            ),
            const SizedBox(height: 20),

            // Текущая информация о боксе
            Padding(
              padding: const EdgeInsets.all(16.0),
              child: Column(
                children: [
                  Text(
                    boxData[_currentIndex]['title']!,
                    style: GoogleFonts.montserrat(
                      fontSize: 22,
                      fontWeight: FontWeight.bold,
                      color: const Color(0xFF4A6E2B),
                    ),
                  ),
                  const SizedBox(
                      height: 8), // Уменьшаем отступ между текстом и описанием
                  Text(
                    boxData[_currentIndex]['description']!,
                    textAlign: TextAlign.center,
                    style: GoogleFonts.openSans(
                      fontSize: 16,
                      color: Colors.black87,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 20), // Уменьшаем пространство до кнопки

            // Кнопка для отправки GET запроса
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 20.0),
              child: GestureDetector(
                onTapDown: (_) {
                  setState(() {
                    _buttonHeight = 55.0; // Уменьшаем размер при нажатии
                  });
                },
                onTapUp: (_) {
                  setState(() {
                    _buttonHeight = 60.0; // Восстанавливаем размер
                  });
                },
                onTapCancel: () {
                  setState(() {
                    _buttonHeight = 60.0; // Восстанавливаем размер при отмене
                  });
                },
                onTap: () async {
                  // Trigger the fetchData function
                  await fetchData();
                },
                child: AnimatedContainer(
                  duration: const Duration(milliseconds: 150),
                  height: _buttonHeight,
                  decoration: BoxDecoration(
                    color: const Color(0xFF6C9942),
                    borderRadius: BorderRadius.circular(30),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black38,
                        blurRadius: 12,
                        offset: const Offset(0, 6),
                      ),
                    ],
                  ),
                  alignment: Alignment.center,
                  child: Text(
                    'Отправить запрос',
                    style: GoogleFonts.montserrat(
                      fontSize: 18, // Увеличиваем размер текста
                      fontWeight: FontWeight.w500,
                      color: Colors.white,
                    ),
                  ),
                ),
              ),
            ),
            const SizedBox(height: 20),

            // Кнопка перехода с анимацией
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 20.0),
              child: GestureDetector(
                onTapDown: (_) {
                  setState(() {
                    _buttonHeight = 55.0; // Уменьшаем размер при нажатии
                  });
                },
                onTapUp: (_) {
                  setState(() {
                    _buttonHeight = 60.0; // Восстанавливаем размер
                  });
                },
                onTapCancel: () {
                  setState(() {
                    _buttonHeight = 60.0; // Восстанавливаем размер при отмене
                  });
                },
                onTap: () {
                  // Переход на экран выбора бокса
                  Navigator.pushNamed(context, '/box-selection');
                },
                child: AnimatedContainer(
                  duration: const Duration(milliseconds: 150),
                  height: _buttonHeight,
                  decoration: BoxDecoration(
                    color: const Color(0xFF6C9942),
                    borderRadius: BorderRadius.circular(30),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black38,
                        blurRadius: 12,
                        offset: const Offset(0, 6),
                      ),
                    ],
                  ),
                  alignment: Alignment.center,
                  child: Text(
                    'Выбрать бокс',
                    style: GoogleFonts.montserrat(
                      fontSize: 18, // Увеличиваем размер текста
                      fontWeight: FontWeight.w500,
                      color: Colors.white,
                    ),
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
