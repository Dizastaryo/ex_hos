import 'dart:async';
import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';
import 'second_screen.dart'; // Подключаем второй экран

class SplashScreen extends StatefulWidget {
  @override
  _SplashScreenState createState() => _SplashScreenState();
}

class _SplashScreenState extends State<SplashScreen> {
  late VideoPlayerController _controller;
  bool _isVideoInitialized = false;

  @override
  void initState() {
    super.initState();
    _initializeVideo();
  }

  void _initializeVideo() {
    _controller = VideoPlayerController.asset('assets/intro.mp4')
      ..initialize().then((_) {
        setState(() {
          _isVideoInitialized = true; // Видео инициализировано
        });
        _controller.play();
        _controller.setLooping(false);

        // Добавляем задержку перед переходом
        _controller.addListener(() {
          if (_controller.value.position == _controller.value.duration) {
            _navigateToNextScreen();
          }
        });
      }).catchError((error) {
        print("Ошибка при инициализации видео: $error");
      });
  }

  Future<void> _navigateToNextScreen() async {
    // Переход на второй экран после завершения видео
    Navigator.pushReplacement(
      context,
      MaterialPageRoute(builder: (context) => SecondScreen()),
    );
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.white, // Устанавливаем белый фон на весь экран
      body: Center(
        child: _isVideoInitialized
            ? AspectRatio(
                aspectRatio: _controller.value.aspectRatio,
                child: VideoPlayer(_controller),
              )
            : CircularProgressIndicator(), // Показываем индикатор загрузки, если видео не инициализировано
      ),
    );
  }
}
