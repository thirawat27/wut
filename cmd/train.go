// Package cmd provides CLI commands for WUT
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"wut/internal/ai"
	"wut/internal/config"
	"wut/internal/db"
	"wut/internal/logger"
	"wut/internal/metrics"
)

// trainCmd represents the train command
var trainCmd = &cobra.Command{
	Use:   "train",
	Short: "Train AI model on your command history",
	Long: `Train the AI model using your command history to improve suggestions.
The model learns from your usage patterns and preferences.`,
	Example: `  wut train
  wut train --epochs 200
  wut train --model personal --learning-rate 0.005`,
	RunE: runTrain,
}

var (
	trainEpochs        int
	trainLearningRate  float64
	trainBatchSize     int
	trainModelName     string
	trainForce         bool
	trainValidate      bool
)

func init() {
	rootCmd.AddCommand(trainCmd)

	trainCmd.Flags().IntVarP(&trainEpochs, "epochs", "e", 100, "number of training epochs")
	trainCmd.Flags().Float64VarP(&trainLearningRate, "learning-rate", "l", 0.01, "learning rate")
	trainCmd.Flags().IntVarP(&trainBatchSize, "batch-size", "b", 32, "batch size")
	trainCmd.Flags().StringVarP(&trainModelName, "model", "m", "default", "model name")
	trainCmd.Flags().BoolVarP(&trainForce, "force", "f", false, "force training even with insufficient data")
	trainCmd.Flags().BoolVarP(&trainValidate, "validate", "v", true, "validate model after training")
}

func runTrain(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	log := logger.With("train")
	start := time.Now()

	log.Info("starting AI model training",
		"epochs", trainEpochs,
		"learning_rate", trainLearningRate,
		"batch_size", trainBatchSize,
		"model", trainModelName,
	)

	cfg := config.Get()

	// Initialize storage
	storage, err := db.NewStorage(cfg.Database.Path)
	if err != nil {
		log.Error("failed to initialize storage", "error", err)
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer storage.Close()

	// Check if we have enough data
	historyCount, err := storage.GetHistoryCount(ctx)
	if err != nil {
		log.Error("failed to get history count", "error", err)
		return fmt.Errorf("failed to get history count: %w", err)
	}

	minEntries := cfg.AI.Training.MinHistoryEntries
	if historyCount < minEntries && !trainForce {
		return fmt.Errorf("insufficient training data: got %d entries, need at least %d (use --force to override)",
			historyCount, minEntries)
	}

	log.Info("training data ready", "entries", historyCount)

	// Get training data
	trainingData, err := storage.GetTrainingData(ctx)
	if err != nil {
		log.Error("failed to get training data", "error", err)
		return fmt.Errorf("failed to get training data: %w", err)
	}

	// Create trainer
	trainer := ai.NewTrainer(ai.TrainingConfig{
		Epochs:        trainEpochs,
		LearningRate:  trainLearningRate,
		BatchSize:     trainBatchSize,
		ModelName:     trainModelName,
		EmbeddingDims: cfg.AI.Model.EmbeddingDimensions,
		HiddenLayers:  cfg.AI.Model.HiddenLayers,
		HiddenUnits:   cfg.AI.Model.HiddenUnits,
	})
	
	// Convert training data
	aiTrainingData := &ai.TrainingData{
		Commands: trainingData.Commands,
		Counts:   trainingData.Counts,
	}

	// Run training
	result, err := trainer.Train(ctx, aiTrainingData)
	if err != nil {
		log.Error("training failed", "error", err)
		return fmt.Errorf("training failed: %w", err)
	}

	trainingTime := time.Since(start)
	log.Info("training completed",
		"duration", trainingTime,
		"final_loss", result.FinalLoss,
		"accuracy", result.Accuracy,
	)

	// Save model
	modelPath := cfg.AI.Model.Path
	if err := trainer.SaveModel(ctx, modelPath, trainModelName); err != nil {
		log.Error("failed to save model", "error", err)
		return fmt.Errorf("failed to save model: %w", err)
	}

	log.Info("model saved", "path", modelPath, "name", trainModelName)

	// Validate if requested
	if trainValidate {
		if err := validateModel(ctx, modelPath, trainModelName, aiTrainingData); err != nil {
			log.Warn("model validation failed", "error", err)
		}
	}

	// Record metrics
	metrics.RecordAITrainingRun()

	// Display results
	displayTrainingResults(result, trainingTime)

	return nil
}

func validateModel(ctx context.Context, modelPath, modelName string, testData *ai.TrainingData) error {
	log := logger.With("train.validate")
	log.Info("validating model")

	model, err := ai.LoadModel(modelPath, modelName)
	if err != nil {
		return fmt.Errorf("failed to load model: %w", err)
	}
	defer model.Close()

	// Run validation
	accuracy, err := model.Validate(ctx, testData)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	log.Info("validation completed", "accuracy", accuracy)
	fmt.Printf("\nValidation Accuracy: %.2f%%\n", accuracy*100)

	return nil
}

func displayTrainingResults(result *ai.TrainingResult, duration time.Duration) {
	fmt.Println()
	fmt.Println("Training Results")
	fmt.Println("================")
	fmt.Printf("Training Time: %s\n", duration)
	fmt.Printf("Epochs: %d\n", result.Epochs)
	fmt.Printf("Final Loss: %.4f\n", result.FinalLoss)
	fmt.Printf("Initial Loss: %.4f\n", result.InitialLoss)
	fmt.Printf("Accuracy: %.2f%%\n", result.Accuracy*100)
	
	if len(result.LossHistory) > 0 {
		fmt.Println("\nLoss History:")
		for i, loss := range result.LossHistory {
			if i%10 == 0 || i == len(result.LossHistory)-1 {
				fmt.Printf("  Epoch %d: %.4f\n", i+1, loss)
			}
		}
	}
	
	fmt.Println()
	fmt.Println("Model saved successfully!")
}
